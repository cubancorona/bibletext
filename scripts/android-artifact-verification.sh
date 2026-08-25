#!/usr/bin/env bash
# Shared signature checks for final Android artifacts.
#
# This file is sourced by build-android.sh and its synthetic regression test.
# Callers must provide private log paths; verifier output can contain signer
# metadata and is never copied to the public release directory.

normalize_android_cert_digest() {
  tr -d ':[:space:]' | tr '[:upper:]' '[:lower:]'
}

verify_android_dex_bridge() {
  local kind="$1"
  local artifact="$2"
  local work_dir="$3"
  local primary secondary listing count required_entry required_bridge

  case "$kind" in
    aab)
      primary=base/dex/classes.dex
      secondary=base/dex/classes2.dex
      ;;
    apk)
      primary=classes.dex
      secondary=classes2.dex
      ;;
    *)
      echo "ERROR: unknown Android artifact kind: $kind" >&2
      return 1
      ;;
  esac

  listing="$work_dir/${kind}-zip-entries.txt"
  if ! unzip -Z1 "$artifact" > "$listing"; then
    echo "ERROR: cannot inspect dex entries in $kind artifact" >&2
    return 1
  fi
  for required_entry in "$primary" "$secondary"; do
    count="$(awk -v entry="$required_entry" '$0 == entry {count++} END {print count+0}' "$listing")"
    if [ "$count" != "1" ]; then
      echo "ERROR: $kind artifact must contain exactly one $required_entry" >&2
      return 1
    fi
  done

  if ! unzip -p "$artifact" "$primary" > "$work_dir/${kind}-classes.dex"; then
    echo "ERROR: cannot extract primary dex from $kind artifact" >&2
    return 1
  fi
  strings "$work_dir/${kind}-classes.dex" > "$work_dir/${kind}-classes.strings"
  if ! grep -Fq 'onNewIntent' "$work_dir/${kind}-classes.strings"; then
    echo "ERROR: final $kind classes.dex has no onNewIntent bridge" >&2
    return 1
  fi
  if ! unzip -p "$artifact" "$secondary" > "$work_dir/${kind}-classes2.dex"; then
    echo "ERROR: cannot extract bridge dex from $kind artifact" >&2
    return 1
  fi
  strings "$work_dir/${kind}-classes2.dex" > "$work_dir/${kind}-classes2.strings"
  for required_bridge in BtBridge BtAudioService; do
    if ! grep -Fq "$required_bridge" "$work_dir/${kind}-classes2.strings"; then
      echo "ERROR: final $kind classes2.dex lacks $required_bridge" >&2
      return 1
    fi
  done
}

verify_android_aab_signature() {
  local artifact="$1"
  local expected_digest="$2"
  local verify_log="$3"
  local cert_log="$4"
  local digest_count actual_digest

  if ! LC_ALL=C jarsigner -verify -verbose -certs "$artifact" >"$verify_log" 2>&1; then
    echo "ERROR: AAB signature verification failed" >&2
    return 1
  fi
  if ! grep -Fq 'jar verified.' "$verify_log" \
     || grep -Fiq 'jar is unsigned' "$verify_log" \
     || grep -Fiq 'unsigned entries' "$verify_log"; then
    echo "ERROR: AAB is unsigned or lacks a verified JAR signature" >&2
    return 1
  fi

  if ! LC_ALL=C keytool -printcert -jarfile "$artifact" >"$cert_log" 2>&1; then
    echo "ERROR: AAB signer certificate could not be read" >&2
    return 1
  fi
  digest_count="$(awk '/^[[:space:]]*SHA256:/{count++} END{print count+0}' "$cert_log")"
  if [ "$digest_count" != "1" ]; then
    echo "ERROR: AAB must have exactly one signer certificate" >&2
    return 1
  fi
  actual_digest="$(awk '/^[[:space:]]*SHA256:/{sub(/^[^:]*:[[:space:]]*/, ""); print; exit}' \
    "$cert_log" | normalize_android_cert_digest)"
  if [ -z "$actual_digest" ] || [ "$actual_digest" != "$expected_digest" ]; then
    echo "ERROR: AAB signer certificate does not match the configured upload key" >&2
    return 1
  fi
}

verify_android_apk_signature() {
  local artifact="$1"
  local expected_digest="$2"
  local apksigner="$3"
  local verify_log="$4"
  local digest_count actual_digest

  if ! LC_ALL=C "$apksigner" verify --verbose --print-certs "$artifact" >"$verify_log" 2>&1; then
    echo "ERROR: APK signature verification failed" >&2
    return 1
  fi
  for required in \
    'Verified using v1 scheme (JAR signing): true' \
    'Verified using v2 scheme (APK Signature Scheme v2): true' \
    'Verified using v3 scheme (APK Signature Scheme v3): true'; do
    if ! grep -Fq "$required" "$verify_log"; then
      echo "ERROR: APK is missing a required Android signature scheme" >&2
      return 1
    fi
  done

  digest_count="$(awk '/^Signer #[0-9]+ certificate SHA-256 digest:/{count++} END{print count+0}' \
    "$verify_log")"
  if [ "$digest_count" != "1" ]; then
    echo "ERROR: APK must have exactly one signer certificate" >&2
    return 1
  fi
  actual_digest="$(awk '/^Signer #1 certificate SHA-256 digest:/{sub(/^[^:]*:[[:space:]]*/, ""); print; exit}' \
    "$verify_log" | normalize_android_cert_digest)"
  if [ -z "$actual_digest" ] || [ "$actual_digest" != "$expected_digest" ]; then
    echo "ERROR: APK signer certificate does not match the configured upload key" >&2
    return 1
  fi
}
