//go:build darwin && !ios

package bibletext

// Native macOS per-chapter audio — the desktop twin of audio_ios.go. Same two
// engines behind one façade:
//   - AVPlayer streams a recorded MP3 (seekable via HTTP range) for chapters that
//     have a recording.
//   - AVSpeechSynthesizer reads the chapter's own verses aloud (TTS) otherwise.
//
// Differences from iOS: macOS has NO AVAudioSession (audio just plays; there is
// no session to activate or interruption to handle) and no UIBackgroundModes —
// a desktop app keeps playing in the background for free. MPNowPlayingInfoCenter +
// MPRemoteCommandCenter still drive the Control Center / media-key Now Playing
// card (macOS 10.12.2+) with play/pause and ±15-second skip (no track skip).
// Artwork uses NSImage (AppKit) instead of UIImage. State changes post back to Go
// via bibleTextAudioStateChanged (audio_export_apple.go), exactly as on iOS.
//
// ARC is on (-fobjc-arc, like reading_macos.go); the controller lives in a strong
// static so KVO/delegate callbacks always have a live target.

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework AVFoundation -framework MediaPlayer -framework CoreMedia -framework AppKit -framework Foundation

#import <AppKit/AppKit.h>
#import <AVFoundation/AVFoundation.h>
#import <MediaPlayer/MediaPlayer.h>
#import <math.h>
#import <stdlib.h>

// Implemented in Go (audio_export_apple.go, //export). Codes: 0 idle, 1 playing,
// 2 paused, 3 ended.
extern void bibleTextAudioStateChanged(int code);

enum { BT_AUDIO_IDLE = 0, BT_AUDIO_PLAYING = 1, BT_AUDIO_PAUSED = 2, BT_AUDIO_ENDED = 3 };
typedef enum { BT_MODE_NONE = 0, BT_MODE_URL = 1, BT_MODE_TTS = 2 } BTAudioMode;

static void btAudioSetupCommands(void);
static void btAudioUpdateNowPlaying(void);

@interface BTAudioController : NSObject <AVSpeechSynthesizerDelegate>
@property (nonatomic, assign) BTAudioMode mode;
@property (nonatomic, strong) AVPlayer *player;
@property (nonatomic, strong) AVPlayerItem *item;
@property (nonatomic, strong) AVSpeechSynthesizer *synth;
@property (nonatomic, copy)   NSString *title;
@property (nonatomic, copy)   NSString *artist;
@property (nonatomic, strong) MPMediaItemArtwork *artwork;
@property (nonatomic, assign) BOOL kvoRegistered;
@property (nonatomic, assign) int  gen;   // bumped on every teardown; cancels stale watchdogs
@end

static BTAudioController *gBTAudio = nil;

static void *kBTStatusCtx = &kBTStatusCtx;
static void *kBTRateCtx   = &kBTRateCtx;

// A remote MP3 buffering is "intended playing", not paused.
static BOOL btTCSIsActive(AVPlayerTimeControlStatus tcs) {
    return tcs == AVPlayerTimeControlStatusPlaying ||
           tcs == AVPlayerTimeControlStatusWaitingToPlayAtSpecifiedRate;
}

@implementation BTAudioController

+ (BTAudioController *)shared {
    if (gBTAudio == nil) {
        gBTAudio = [[BTAudioController alloc] init];
        gBTAudio.mode = BT_MODE_NONE;
        // No AVAudioSession interruption observer on macOS — there is no audio
        // session, so nothing interrupts playback the way a phone call does on iOS.
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

    btAudioSetupCommands();
    int g = self.gen;
    [p play];
    btAudioUpdateNowPlaying();

    // Watchdog: if the stream never gets going (dead network), give up so the glyph
    // reverts instead of showing pause forever. A later start/stop bumps gen.
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

    btAudioSetupCommands();   // disables the skip arrows for TTS (can't seek)
    [self.synth speakUtterance:u];
    btAudioUpdateNowPlaying();
}

- (void)toggle {
    if (self.mode == BT_MODE_URL) {
        if (self.player.timeControlStatus == AVPlayerTimeControlStatusPaused) {
            [self.player play];
        } else {
            [self.player pause];
        }
        btAudioUpdateNowPlaying();
    } else if (self.mode == BT_MODE_TTS) {
        if (self.synth.isPaused) {
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
    CMTime tol = CMTimeMakeWithSeconds(0.5, NSEC_PER_SEC);
    [self.player seekToTime:tt toleranceBefore:tol toleranceAfter:tol
          completionHandler:^(BOOL finished){ btAudioUpdateNowPlaying(); }];
}

- (BOOL)isPlaying {
    if (self.mode == BT_MODE_URL) {
        return btTCSIsActive(self.player.timeControlStatus);
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
            bibleTextAudioStateChanged(BT_AUDIO_PLAYING);
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

// AVSpeechSynthesizerDelegate. Gated on mode==TTS so a stale callback from a synth
// we've switched away from can't post a spurious state (see audio_ios.go).
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

    cc.nextTrackCommand.enabled = NO;
    cc.previousTrackCommand.enabled = NO;

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
    MPNowPlayingInfoCenter *npc = [MPNowPlayingInfoCenter defaultCenter];
    if (c.mode == BT_MODE_NONE) {
        npc.playbackState = MPNowPlayingPlaybackStateStopped;
        npc.nowPlayingInfo = nil;
        return;
    }
    NSMutableDictionary *info = [NSMutableDictionary dictionary];
    if (c.title)   info[MPMediaItemPropertyTitle]    = c.title;
    if (c.artist)  info[MPMediaItemPropertyArtist]   = c.artist;
    if (c.artwork) info[MPMediaItemPropertyArtwork]  = c.artwork;

    BOOL playing = NO;
    if (c.mode == BT_MODE_URL && c.player && c.item) {
        double elapsed = CMTimeGetSeconds(c.player.currentTime);
        if (!isfinite(elapsed)) elapsed = 0.0;
        CMTime dur = c.item.duration;
        double durS = CMTIME_IS_NUMERIC(dur) ? CMTimeGetSeconds(dur) : 0.0;
        playing = btTCSIsActive(c.player.timeControlStatus);
        info[MPNowPlayingInfoPropertyElapsedPlaybackTime] = @(elapsed);
        if (durS > 0.0) info[MPMediaItemPropertyPlaybackDuration] = @(durS);
        info[MPNowPlayingInfoPropertyPlaybackRate] = @(playing ? 1.0 : 0.0);
    } else if (c.mode == BT_MODE_TTS) {
        playing = (c.synth.isSpeaking && !c.synth.isPaused);
        info[MPNowPlayingInfoPropertyPlaybackRate] = @(playing ? 1.0 : 0.0);
    }
    // macOS has no audio session for the system to infer play/pause from, so set
    // playbackState explicitly — otherwise the Control Center card / media keys
    // can look inert (the in-app button is driven by the Go callbacks regardless).
    npc.playbackState = playing ? MPNowPlayingPlaybackStatePlaying : MPNowPlayingPlaybackStatePaused;
    npc.nowPlayingInfo = info;
}

// ---- C surface called from Go. Copy strings, then run AVFoundation on main. ----
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
        [MPNowPlayingInfoCenter defaultCenter].playbackState = MPNowPlayingPlaybackStateStopped;
        [MPNowPlayingInfoCenter defaultCenter].nowPlayingInfo = nil;
        // No AVAudioSession to deactivate on macOS.
    });
}

// Set the Now Playing artwork from a PNG file (rendered in Go). NSImage on macOS.
void bibleTextAudioSetArtwork(const char *path) {
    NSString *p = path ? [NSString stringWithUTF8String:path] : @"";
    dispatch_async(dispatch_get_main_queue(), ^{
        NSImage *img = [[NSImage alloc] initWithContentsOfFile:p];
        if (img == nil) return;
        BTAudioController *c = [BTAudioController shared];
        c.artwork = [[MPMediaItemArtwork alloc] initWithBoundsSize:img.size
            requestHandler:^NSImage *(CGSize sz){ return img; }];
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
