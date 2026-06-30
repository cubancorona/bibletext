//go:build ios

package bibletext

// Native iOS per-chapter audio. Two engines behind one façade:
//   - AVPlayer streams a recorded MP3 (seekable via HTTP range) for WEB chapters
//     eBible has recorded.
//   - AVSpeechSynthesizer reads the chapter's own verses aloud (TTS) for every
//     other version / book — always matching the on-screen text exactly.
//
// AVAudioSession(.playback) keeps audio going when the app is backgrounded and
// over the silent switch (needs UIBackgroundModes=audio in Info.plist — injected
// by scripts/run-ios-device.sh / release-ios.sh). MPNowPlayingInfoCenter +
// MPRemoteCommandCenter drive the lock-screen / Control Center Now Playing card
// with play/pause and ±15-second skip arrows (NOT next/previous-track). State
// changes post back to Go via bibleTextAudioStateChanged (audio_export_ios.go) so
// the play button updates.
//
// ARC is on (-fobjc-arc, like reading_ios.go): strong properties, no manual
// retain/release. The controller lives in a strong static so it outlives every C
// call; KVO/notification observers are torn down before the observed objects.
// CoreMedia is linked for CMTime; AVFoundation hosts AVAudioSession on iOS.

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AVFoundation -framework MediaPlayer -framework CoreMedia -framework UIKit -framework Foundation

#import <UIKit/UIKit.h>
#import <AVFoundation/AVFoundation.h>
#import <MediaPlayer/MediaPlayer.h>
#import <math.h>
#import <stdlib.h>

// Implemented in Go (audio_export_ios.go, //export). Posted whenever playback
// state changes on its own — chapter finished, a phone-call interruption, or a
// lock-screen / Control Center toggle. Codes: 0 idle, 1 playing, 2 paused, 3 ended.
extern void bibleTextAudioStateChanged(int code);

enum { BT_AUDIO_IDLE = 0, BT_AUDIO_PLAYING = 1, BT_AUDIO_PAUSED = 2, BT_AUDIO_ENDED = 3 };
typedef enum { BT_MODE_NONE = 0, BT_MODE_URL = 1, BT_MODE_TTS = 2 } BTAudioMode;

static BOOL btAudioSetupSession(void);
static void btAudioSetupCommands(void);
static void btAudioUpdateNowPlaying(void);

@interface BTAudioController : NSObject <AVSpeechSynthesizerDelegate>
@property (nonatomic, assign) BTAudioMode mode;
@property (nonatomic, strong) AVPlayer *player;
@property (nonatomic, strong) AVPlayerItem *item;
@property (nonatomic, strong) AVSpeechSynthesizer *synth;
@property (nonatomic, copy)   NSString *title;
@property (nonatomic, copy)   NSString *artist;
@property (nonatomic, strong) MPMediaItemArtwork *artwork;   // lock-screen card; persists across now-playing refreshes
@property (nonatomic, assign) BOOL kvoRegistered;
@property (nonatomic, assign) int  gen;   // bumped on every teardown; cancels stale watchdogs
@end

// The one, forever-retained controller (strong static → ARC keeps it alive, so
// KVO/delegate callbacks always have a live target).
static BTAudioController *gBTAudio = nil;

// Unique KVO context addresses.
static void *kBTStatusCtx = &kBTStatusCtx;
static void *kBTRateCtx   = &kBTRateCtx;

// A remote MP3 buffering is "intended playing", not paused — used so the glyph and
// Now Playing don't flip to paused mid-buffer.
static BOOL btTCSIsActive(AVPlayerTimeControlStatus tcs) {
    return tcs == AVPlayerTimeControlStatusPlaying ||
           tcs == AVPlayerTimeControlStatusWaitingToPlayAtSpecifiedRate;
}

@implementation BTAudioController

+ (BTAudioController *)shared {
    if (gBTAudio == nil) {
        gBTAudio = [[BTAudioController alloc] init];
        gBTAudio.mode = BT_MODE_NONE;
        // One lifetime interruption observer (phone call etc.). Not per-item, so it
        // is never removed in teardownEngines.
        [[NSNotificationCenter defaultCenter] addObserver:gBTAudio
            selector:@selector(btHandleInterruption:)
            name:AVAudioSessionInterruptionNotification
            object:[AVAudioSession sharedInstance]];
    }
    return gBTAudio;
}

// ---- recorded MP3 via AVPlayer (HTTP range streaming + seek) ----
- (void)startURL:(NSString *)urlStr title:(NSString *)t artist:(NSString *)a {
    [self teardownEngines];
    self.mode = BT_MODE_URL;
    self.title = t; self.artist = a;

    NSURL *url = [NSURL URLWithString:urlStr];
    if (url == nil) { self.mode = BT_MODE_NONE; bibleTextAudioStateChanged(BT_AUDIO_IDLE); return; }

    AVPlayerItem *it = [AVPlayerItem playerItemWithURL:url];
    self.item = it;
    AVPlayer *p = [AVPlayer playerWithPlayerItem:it];
    p.automaticallyWaitsToMinimizeStalling = YES;
    self.player = p;

    [it addObserver:self forKeyPath:@"status"
            options:NSKeyValueObservingOptionNew context:kBTStatusCtx];
    [p addObserver:self forKeyPath:@"timeControlStatus"
            options:NSKeyValueObservingOptionNew context:kBTRateCtx];
    self.kvoRegistered = YES;

    [[NSNotificationCenter defaultCenter] addObserver:self
        selector:@selector(btItemDidEnd:)
        name:AVPlayerItemDidPlayToEndTimeNotification object:it];

    if (!btAudioSetupSession()) {   // activate BEFORE play; bail (glyph reverts) on failure
        [self teardownEngines];
        bibleTextAudioStateChanged(BT_AUDIO_IDLE);
        return;
    }
    btAudioSetupCommands();
    int g = self.gen;   // teardownEngines (above) already bumped gen; capture the live one
    [p play];
    btAudioUpdateNowPlaying();

    // Watchdog: if the stream never gets going (dead network / hung connection),
    // give up after a generous window so the glyph reverts instead of showing pause
    // forever. A 404 already surfaces as AVPlayerItemStatusFailed → IDLE; this is the
    // backstop for a silent stall. A later start/stop bumps gen and cancels this.
    dispatch_after(dispatch_time(DISPATCH_TIME_NOW, (int64_t)(30.0 * NSEC_PER_SEC)),
                   dispatch_get_main_queue(), ^{
        BTAudioController *c = [BTAudioController shared];
        if (c.gen != g || c.mode != BT_MODE_URL) return;   // superseded
        if (c.item.status != AVPlayerItemStatusReadyToPlay &&
            c.player.timeControlStatus != AVPlayerTimeControlStatusPlaying) {
            [c teardownEngines];
            bibleTextAudioStateChanged(BT_AUDIO_IDLE);
        }
    });
}

// ---- on-device TTS via AVSpeechSynthesizer ----
- (void)startTTS:(NSString *)text title:(NSString *)t artist:(NSString *)a {
    [self teardownEngines];
    self.mode = BT_MODE_TTS;
    self.title = t; self.artist = a;

    if (self.synth == nil) {
        self.synth = [[AVSpeechSynthesizer alloc] init];
        self.synth.delegate = self;
    }
    AVSpeechUtterance *u = [AVSpeechUtterance speechUtteranceWithString:text];
    u.voice = [AVSpeechSynthesisVoice voiceWithLanguage:@"en-US"];
    u.rate = AVSpeechUtteranceDefaultSpeechRate;
    u.pitchMultiplier = 1.0;
    u.volume = 1.0;

    if (!btAudioSetupSession()) {
        [self teardownEngines];
        bibleTextAudioStateChanged(BT_AUDIO_IDLE);
        return;
    }
    btAudioSetupCommands();   // disables the skip arrows for TTS (can't seek)
    [self.synth speakUtterance:u];
    btAudioUpdateNowPlaying();
}

- (void)toggle {
    if (self.mode == BT_MODE_URL) {
        if (self.player.timeControlStatus == AVPlayerTimeControlStatusPaused) {
            btAudioSetupSession();   // re-activate in case an interruption deactivated us
            [self.player play];
        } else {
            [self.player pause];
        }
        btAudioUpdateNowPlaying();
    } else if (self.mode == BT_MODE_TTS) {
        if (self.synth.isPaused) {
            btAudioSetupSession();
            [self.synth continueSpeaking];
        } else if (self.synth.isSpeaking) {
            [self.synth pauseSpeakingAtBoundary:AVSpeechBoundaryWord];
        }
        btAudioUpdateNowPlaying();
    }
}

- (void)skip:(double)seconds {
    if (self.mode != BT_MODE_URL) return;   // TTS can't seek
    CMTime cur = self.player.currentTime;
    CMTime dur = self.item.duration;
    double curS = CMTIME_IS_NUMERIC(cur) ? CMTimeGetSeconds(cur) : 0.0;
    double durS = CMTIME_IS_NUMERIC(dur) ? CMTimeGetSeconds(dur) : 0.0;
    double target = curS + seconds;
    if (target < 0.0) target = 0.0;
    if (durS > 0.0 && target > durS) target = durS;
    CMTime tt = CMTimeMakeWithSeconds(target, NSEC_PER_SEC);
    // A small tolerance lets the player snap to the nearest buffered sync sample
    // instead of forcing a frame-accurate re-fetch (which re-buffers a remote MP3).
    CMTime tol = CMTimeMakeWithSeconds(0.5, NSEC_PER_SEC);
    [self.player seekToTime:tt toleranceBefore:tol toleranceAfter:tol
          completionHandler:^(BOOL finished){ btAudioUpdateNowPlaying(); }];
}

- (BOOL)isPlaying {
    if (self.mode == BT_MODE_URL) {
        return btTCSIsActive(self.player.timeControlStatus);   // buffering counts as playing
    }
    if (self.mode == BT_MODE_TTS) {
        return self.synth.isSpeaking && !self.synth.isPaused;
    }
    return NO;
}

- (void)observeValueForKeyPath:(NSString *)keyPath ofObject:(id)object
                        change:(NSDictionary *)change context:(void *)context {
    if (context == kBTStatusCtx) {
        if (self.item.status == AVPlayerItemStatusReadyToPlay) {
            btAudioUpdateNowPlaying();   // duration now known
        } else if (self.item.status == AVPlayerItemStatusFailed) {
            bibleTextAudioStateChanged(BT_AUDIO_IDLE);
        }
    } else if (context == kBTRateCtx) {
        AVPlayerTimeControlStatus tcs = self.player.timeControlStatus;
        if (btTCSIsActive(tcs)) {
            bibleTextAudioStateChanged(BT_AUDIO_PLAYING);   // includes WaitingToPlay (buffering)
        } else if (tcs == AVPlayerTimeControlStatusPaused) {
            bibleTextAudioStateChanged(BT_AUDIO_PAUSED);
        }
        btAudioUpdateNowPlaying();
    } else {
        [super observeValueForKeyPath:keyPath ofObject:object change:change context:context];
    }
}

- (void)btItemDidEnd:(NSNotification *)n {
    if (self.mode != BT_MODE_URL) return;   // ignore a stale end from a torn-down player
    bibleTextAudioStateChanged(BT_AUDIO_ENDED);
    btAudioUpdateNowPlaying();
}

- (void)btHandleInterruption:(NSNotification *)n {
    NSNumber *type = n.userInfo[AVAudioSessionInterruptionTypeKey];
    if (type == nil) return;
    NSUInteger t = type.unsignedIntegerValue;
    NSNumber *optNum = n.userInfo[AVAudioSessionInterruptionOptionKey];
    NSUInteger opts = optNum ? optNum.unsignedIntegerValue : 0;
    // The notification can arrive off the main thread; all the AVFoundation /
    // MediaPlayer work below must run on main.
    dispatch_async(dispatch_get_main_queue(), ^{
        BTAudioController *c = [BTAudioController shared];
        if (t == AVAudioSessionInterruptionTypeBegan) {
            // The system paused us (phone call, etc.); reflect it on the button.
            bibleTextAudioStateChanged(BT_AUDIO_PAUSED);
        } else if (t == AVAudioSessionInterruptionTypeEnded) {
            if ((opts & AVAudioSessionInterruptionOptionShouldResume) && c.mode != BT_MODE_NONE) {
                // The session was deactivated by the interruption — re-activate, then resume.
                if (btAudioSetupSession()) {
                    if (c.mode == BT_MODE_URL) {
                        [c.player play];
                    } else if (c.mode == BT_MODE_TTS) {
                        [c.synth continueSpeaking];
                    }
                    bibleTextAudioStateChanged(BT_AUDIO_PLAYING);
                    btAudioUpdateNowPlaying();
                }
            }
        }
    });
}

// AVSpeechSynthesizerDelegate. Every callback is gated on mode==TTS: when the
// reader switches to a recorded narration, teardownEngines stops the synth but
// (unlike the AVPlayer's KVO observer) the synth delegate stays wired, so a
// stopped utterance's didFinish/didCancel can still fire LATE — after the new
// AVPlayer has started. Posting that stale ENDED would wipe the freshly-loaded
// chapter and desync the play/pause button. The mode guard drops it.
- (void)speechSynthesizer:(AVSpeechSynthesizer *)s didStartSpeechUtterance:(AVSpeechUtterance *)u {
    if (self.mode != BT_MODE_TTS) return;
    bibleTextAudioStateChanged(BT_AUDIO_PLAYING);
}
- (void)speechSynthesizer:(AVSpeechSynthesizer *)s didFinishSpeechUtterance:(AVSpeechUtterance *)u {
    if (self.mode != BT_MODE_TTS) return;
    bibleTextAudioStateChanged(BT_AUDIO_ENDED);
    btAudioUpdateNowPlaying();
}
- (void)speechSynthesizer:(AVSpeechSynthesizer *)s didCancelSpeechUtterance:(AVSpeechUtterance *)u {
    // stopSpeakingAtBoundary fires this (not didFinish) on some iOS versions; same
    // staleness risk, so it's gated identically and treated as a clean stop.
    if (self.mode != BT_MODE_TTS) return;
    bibleTextAudioStateChanged(BT_AUDIO_ENDED);
    btAudioUpdateNowPlaying();
}
- (void)speechSynthesizer:(AVSpeechSynthesizer *)s didPauseSpeechUtterance:(AVSpeechUtterance *)u {
    if (self.mode != BT_MODE_TTS) return;
    bibleTextAudioStateChanged(BT_AUDIO_PAUSED);
    btAudioUpdateNowPlaying();
}
- (void)speechSynthesizer:(AVSpeechSynthesizer *)s didContinueSpeechUtterance:(AVSpeechUtterance *)u {
    if (self.mode != BT_MODE_TTS) return;
    bibleTextAudioStateChanged(BT_AUDIO_PLAYING);
    btAudioUpdateNowPlaying();
}

- (void)teardownEngines {
    self.gen++;   // invalidate any pending start watchdog
    if (self.kvoRegistered) {
        @try { [self.item removeObserver:self forKeyPath:@"status" context:kBTStatusCtx]; }
        @catch (__unused NSException *e) {}
        @try { [self.player removeObserver:self forKeyPath:@"timeControlStatus" context:kBTRateCtx]; }
        @catch (__unused NSException *e) {}
        self.kvoRegistered = NO;
    }
    [[NSNotificationCenter defaultCenter] removeObserver:self
        name:AVPlayerItemDidPlayToEndTimeNotification object:nil];

    if (self.player) { [self.player pause]; self.player = nil; }
    self.item = nil;
    if (self.synth != nil && self.synth.isSpeaking) {
        [self.synth stopSpeakingAtBoundary:AVSpeechBoundaryImmediate];
    }
    self.artwork = nil;
    self.mode = BT_MODE_NONE;
}
@end

// ---- AVAudioSession: playback category, activate before play. Returns success so
//      callers can bail (and revert the glyph) instead of starting silently. ----
static BOOL btAudioSetupSession(void) {
    AVAudioSession *sess = [AVAudioSession sharedInstance];
    NSError *err = nil;
    if (![sess setCategory:AVAudioSessionCategoryPlayback
                      mode:AVAudioSessionModeSpokenAudio
                   options:0 error:&err]) {
        NSLog(@"BibleText audio: setCategory failed: %@", err);
        return NO;
    }
    err = nil;
    if (![sess setActive:YES withOptions:0 error:&err]) {
        NSLog(@"BibleText audio: setActive failed: %@", err);
        return NO;
    }
    return YES;
}

// ---- MPRemoteCommandCenter: play/pause/toggle + ±15s skip, no track skip ----
static void btAudioSetupCommands(void) {
    MPRemoteCommandCenter *cc = [MPRemoteCommandCenter sharedCommandCenter];
    BOOL isURL = ([BTAudioController shared].mode == BT_MODE_URL);

    cc.playCommand.enabled = YES;
    [cc.playCommand removeTarget:nil];
    [cc.playCommand addTargetWithHandler:^MPRemoteCommandHandlerStatus(MPRemoteCommandEvent *e){
        [[BTAudioController shared] toggle];
        return MPRemoteCommandHandlerStatusSuccess;
    }];

    cc.pauseCommand.enabled = YES;
    [cc.pauseCommand removeTarget:nil];
    [cc.pauseCommand addTargetWithHandler:^MPRemoteCommandHandlerStatus(MPRemoteCommandEvent *e){
        [[BTAudioController shared] toggle];
        return MPRemoteCommandHandlerStatusSuccess;
    }];

    cc.togglePlayPauseCommand.enabled = YES;
    [cc.togglePlayPauseCommand removeTarget:nil];
    [cc.togglePlayPauseCommand addTargetWithHandler:^MPRemoteCommandHandlerStatus(MPRemoteCommandEvent *e){
        [[BTAudioController shared] toggle];
        return MPRemoteCommandHandlerStatusSuccess;
    }];

    cc.skipForwardCommand.preferredIntervals = @[@15];
    cc.skipForwardCommand.enabled = isURL;
    [cc.skipForwardCommand removeTarget:nil];
    [cc.skipForwardCommand addTargetWithHandler:^MPRemoteCommandHandlerStatus(MPRemoteCommandEvent *e){
        MPSkipIntervalCommandEvent *se = (MPSkipIntervalCommandEvent *)e;
        [[BTAudioController shared] skip:se.interval];
        return MPRemoteCommandHandlerStatusSuccess;
    }];

    cc.skipBackwardCommand.preferredIntervals = @[@15];
    cc.skipBackwardCommand.enabled = isURL;
    [cc.skipBackwardCommand removeTarget:nil];
    [cc.skipBackwardCommand addTargetWithHandler:^MPRemoteCommandHandlerStatus(MPRemoteCommandEvent *e){
        MPSkipIntervalCommandEvent *se = (MPSkipIntervalCommandEvent *)e;
        [[BTAudioController shared] skip:-se.interval];
        return MPRemoteCommandHandlerStatusSuccess;
    }];

    // No next/previous-track glyphs — we want the skip arrows instead.
    cc.nextTrackCommand.enabled = NO;
    cc.previousTrackCommand.enabled = NO;

    // Lock-screen scrubbing (recorded only).
    cc.changePlaybackPositionCommand.enabled = isURL;
    [cc.changePlaybackPositionCommand removeTarget:nil];
    [cc.changePlaybackPositionCommand addTargetWithHandler:^MPRemoteCommandHandlerStatus(MPRemoteCommandEvent *e){
        BTAudioController *c = [BTAudioController shared];
        if (c.mode != BT_MODE_URL) return MPRemoteCommandHandlerStatusCommandFailed;
        MPChangePlaybackPositionCommandEvent *pe = (MPChangePlaybackPositionCommandEvent *)e;
        CMTime tt = CMTimeMakeWithSeconds(pe.positionTime, NSEC_PER_SEC);
        [c.player seekToTime:tt toleranceBefore:kCMTimeZero toleranceAfter:kCMTimeZero
            completionHandler:^(BOOL f){ btAudioUpdateNowPlaying(); }];
        return MPRemoteCommandHandlerStatusSuccess;
    }];
}

// ---- MPNowPlayingInfoCenter: title, translation, elapsed/duration, rate ----
static void btAudioUpdateNowPlaying(void) {
    BTAudioController *c = [BTAudioController shared];
    if (c.mode == BT_MODE_NONE) {
        // Nothing loaded (e.g. a late artwork callback arriving after stop) — leave
        // the lock screen clear instead of resurrecting a ghost card.
        [MPNowPlayingInfoCenter defaultCenter].nowPlayingInfo = nil;
        return;
    }
    NSMutableDictionary *info = [NSMutableDictionary dictionary];
    if (c.title)   info[MPMediaItemPropertyTitle]    = c.title;
    if (c.artist)  info[MPMediaItemPropertyArtist]   = c.artist;
    if (c.artwork) info[MPMediaItemPropertyArtwork]  = c.artwork;   // survives every refresh

    if (c.mode == BT_MODE_URL && c.player && c.item) {
        double elapsed = CMTimeGetSeconds(c.player.currentTime);
        if (!isfinite(elapsed)) elapsed = 0.0;
        CMTime dur = c.item.duration;
        double durS = CMTIME_IS_NUMERIC(dur) ? CMTimeGetSeconds(dur) : 0.0;
        BOOL playing = btTCSIsActive(c.player.timeControlStatus);   // buffering shows as playing
        info[MPNowPlayingInfoPropertyElapsedPlaybackTime] = @(elapsed);
        if (durS > 0.0) info[MPMediaItemPropertyPlaybackDuration] = @(durS);
        info[MPNowPlayingInfoPropertyPlaybackRate] = @(playing ? 1.0 : 0.0);
    } else if (c.mode == BT_MODE_TTS) {
        // No reliable clock/duration for speech — omit duration (no scrubber), just
        // report the rate so the card shows play/pause correctly.
        BOOL playing = (c.synth.isSpeaking && !c.synth.isPaused);
        info[MPNowPlayingInfoPropertyPlaybackRate] = @(playing ? 1.0 : 0.0);
    }
    [MPNowPlayingInfoCenter defaultCenter].nowPlayingInfo = info;
}

// ---- C surface called from Go. Copy strings here (before the async hop), then
//      run AVFoundation work on the main thread. dispatch_async never blocks, so
//      it is safe even when Go calls from a teardown path. ----
void bibleTextAudioStartURL(const char *url, const char *title, const char *artist) {
    NSString *u = url ? [NSString stringWithUTF8String:url] : @"";
    NSString *t = title ? [NSString stringWithUTF8String:title] : @"";
    NSString *a = artist ? [NSString stringWithUTF8String:artist] : @"";
    dispatch_async(dispatch_get_main_queue(), ^{ [[BTAudioController shared] startURL:u title:t artist:a]; });
}

void bibleTextAudioStartTTS(const char *text, const char *title, const char *artist) {
    NSString *x = text ? [NSString stringWithUTF8String:text] : @"";
    NSString *t = title ? [NSString stringWithUTF8String:title] : @"";
    NSString *a = artist ? [NSString stringWithUTF8String:artist] : @"";
    dispatch_async(dispatch_get_main_queue(), ^{ [[BTAudioController shared] startTTS:x title:t artist:a]; });
}

void bibleTextAudioToggle(void) {
    dispatch_async(dispatch_get_main_queue(), ^{ [[BTAudioController shared] toggle]; });
}

void bibleTextAudioSkip(double seconds) {
    dispatch_async(dispatch_get_main_queue(), ^{ [[BTAudioController shared] skip:seconds]; });
}

void bibleTextAudioStop(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        BTAudioController *c = [BTAudioController shared];
        [c teardownEngines];
        [MPNowPlayingInfoCenter defaultCenter].nowPlayingInfo = nil;
        NSError *err = nil;
        [[AVAudioSession sharedInstance] setActive:NO
            withOptions:AVAudioSessionSetActiveOptionNotifyOthersOnDeactivation error:&err];
        // No state callback here: an explicit stop is always driven from Go
        // (audioController.stop already set idle), so re-entering Go via fyne.Do —
        // which would be undesirable on the shutdown path — is unnecessary.
    });
}

// Set the lock-screen / Control Center artwork from a PNG file (rendered in Go).
// Stored on the controller so it survives the periodic now-playing refreshes; a
// late call after stop is absorbed by the mode==NONE guard in btAudioUpdateNowPlaying.
void bibleTextAudioSetArtwork(const char *path) {
    NSString *p = path ? [NSString stringWithUTF8String:path] : @"";
    dispatch_async(dispatch_get_main_queue(), ^{
        UIImage *img = [UIImage imageWithContentsOfFile:p];
        if (img == nil) return;
        BTAudioController *c = [BTAudioController shared];
        c.artwork = [[MPMediaItemArtwork alloc] initWithBoundsSize:img.size
            requestHandler:^UIImage *(CGSize sz){ return img; }];
        btAudioUpdateNowPlaying();
    });
}
*/
import "C"

import "unsafe"

func nativeAudioStartURL(url, title, artist string) {
	cu := C.CString(url)
	ct := C.CString(title)
	ca := C.CString(artist)
	defer C.free(unsafe.Pointer(cu))
	defer C.free(unsafe.Pointer(ct))
	defer C.free(unsafe.Pointer(ca))
	C.bibleTextAudioStartURL(cu, ct, ca)
}

func nativeAudioStartTTS(text, title, artist string) {
	cx := C.CString(text)
	ct := C.CString(title)
	ca := C.CString(artist)
	defer C.free(unsafe.Pointer(cx))
	defer C.free(unsafe.Pointer(ct))
	defer C.free(unsafe.Pointer(ca))
	C.bibleTextAudioStartTTS(cx, ct, ca)
}

func nativeAudioToggle()              { C.bibleTextAudioToggle() }
func nativeAudioStop()                { C.bibleTextAudioStop() }
func nativeAudioSkip(seconds float64) { C.bibleTextAudioSkip(C.double(seconds)) }

func nativeAudioSetArtwork(path string) {
	cp := C.CString(path)
	defer C.free(unsafe.Pointer(cp))
	C.bibleTextAudioSetArtwork(cp)
}
