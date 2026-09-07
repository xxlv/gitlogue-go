// Package animator turns structured diffs into cinematic typing Action streams.
//
// Step 1 only reserves the package boundary. Step 2 will parse Hunks into
// MoveCursor / TypeChar / DeleteChar / InsertLine instructions and apply a
// Gaussian typing-delay model (30–60 ms for ordinary characters, 150–300 ms
// pauses on punctuation and newlines).
package animator
