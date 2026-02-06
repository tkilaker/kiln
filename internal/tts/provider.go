package tts

import "context"

// Voice represents a TTS voice option
type Voice struct {
	ID   string // Provider-specific voice identifier
	Name string // Human-readable display name
}

// Provider defines the interface for TTS providers (OpenAI, ElevenLabs, etc.)
type Provider interface {
	// GenerateChunkAudio converts a single chunk of text to audio bytes.
	GenerateChunkAudio(ctx context.Context, text, voiceID string) ([]byte, error)

	// AvailableVoices returns the list of voices available from this provider.
	AvailableVoices() []Voice

	// DefaultVoice returns the default voice ID for this provider.
	DefaultVoice() string

	// MaxChunkSize returns the maximum characters per API call.
	MaxChunkSize() int

	// Name returns the provider name (e.g., "openai", "elevenlabs").
	Name() string
}
