//go:build js && wasm

package main

import "syscall/js"

// playAudio forwards raw PCM bytes to the JavaScript audio scheduler for a specific session.
func playAudio(sessionId string, sampleRate, channels, bitsPerSample int, data []byte) {
	jsArr := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(jsArr, data)
	js.Global().Call("rdpAudioPlay", sessionId, sampleRate, channels, bitsPerSample, jsArr)
}
