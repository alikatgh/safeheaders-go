package drwavgo

import "testing"

// FuzzParse ensures Parse and every accessor survive arbitrary input without
// panicking (e.g. divide-by-zero on a zero NumChannels / bit depth).
func FuzzParse(f *testing.F) {
	valid, _ := Serialize(&WAV{
		Header: WAVHeader{
			AudioFormat: 1, NumChannels: 2, SampleRate: 44100,
			ByteRate: 176400, BlockAlign: 4, BitsPerSample: 16,
		},
		Data: []byte{1, 2, 3, 4, 5, 6, 7, 8},
	})
	f.Add(valid)
	f.Add([]byte{})
	f.Add([]byte("RIFF"))
	f.Add(make([]byte, 44))
	// A header that parses but has NumChannels = 0 (the divide-by-zero case).
	zeroCh, _ := Serialize(&WAV{Header: WAVHeader{AudioFormat: 1, BitsPerSample: 16}, Data: []byte{1, 2, 3, 4}})
	f.Add(zeroCh)

	f.Fuzz(func(_ *testing.T, data []byte) {
		wav, err := Parse(data)
		if err != nil || wav == nil {
			return
		}
		// None of these may panic on a successfully-parsed (but possibly
		// malformed) WAV.
		_ = ValidateWAV(wav)
		_ = wav.GetDuration()
		_ = wav.GetSampleCount()
		_, _ = wav.ExtractChannels()
		_, _ = Serialize(wav)
	})
}
