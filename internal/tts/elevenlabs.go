package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	elevenLabsBaseURL    = "https://api.elevenlabs.io/v1/text-to-speech"
	elevenLabsMaxChunk   = 5000 // ElevenLabs supports up to 5000 chars per request
	elevenLabsDefaultModel = "eleven_multilingual_v2" // Best for non-English (Swedish)
)

// ElevenLabsProvider implements the Provider interface for ElevenLabs TTS API.
// Supports custom cloned voices via voice IDs.
type ElevenLabsProvider struct {
	apiKey   string
	model    string
	voiceID  string // The cloned voice ID from ElevenLabs
	voiceName string // Display name for the custom voice
	client   *http.Client
}

// NewElevenLabsProvider creates a new ElevenLabs TTS provider.
// voiceID is the ID of the cloned voice from the ElevenLabs dashboard.
// voiceName is an optional display name (defaults to "Custom Voice").
func NewElevenLabsProvider(apiKey, model, voiceID, voiceName string) *ElevenLabsProvider {
	if model == "" {
		model = elevenLabsDefaultModel
	}
	if voiceName == "" {
		voiceName = "Custom Voice"
	}
	return &ElevenLabsProvider{
		apiKey:    apiKey,
		model:     model,
		voiceID:   voiceID,
		voiceName: voiceName,
		client:    &http.Client{},
	}
}

// elevenLabsRequest is the request body for the ElevenLabs TTS API
type elevenLabsRequest struct {
	Text          string                `json:"text"`
	ModelID       string                `json:"model_id"`
	VoiceSettings elevenLabsVoiceSettings `json:"voice_settings"`
}

type elevenLabsVoiceSettings struct {
	Stability       float64 `json:"stability"`
	SimilarityBoost float64 `json:"similarity_boost"`
	Style           float64 `json:"style,omitempty"`
}

func (p *ElevenLabsProvider) GenerateChunkAudio(ctx context.Context, text, voiceID string) ([]byte, error) {
	if voiceID == "" || voiceID == "custom" {
		voiceID = p.voiceID
	}

	reqBody := elevenLabsRequest{
		Text:    text,
		ModelID: p.model,
		VoiceSettings: elevenLabsVoiceSettings{
			Stability:       0.5,
			SimilarityBoost: 0.75,
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/%s?output_format=mp3_44100_128", elevenLabsBaseURL, voiceID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("xi-api-key", p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "audio/mpeg")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ElevenLabs API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ElevenLabs API returned status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

func (p *ElevenLabsProvider) AvailableVoices() []Voice {
	return []Voice{
		{ID: "custom", Name: p.voiceName},
	}
}

func (p *ElevenLabsProvider) DefaultVoice() string {
	return "custom"
}

func (p *ElevenLabsProvider) MaxChunkSize() int {
	return elevenLabsMaxChunk
}

func (p *ElevenLabsProvider) Name() string {
	return "elevenlabs"
}
