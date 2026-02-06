package tts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const (
	openAITTSURL = "https://api.openai.com/v1/audio/speech"
	maxChunkSize = 4000 // OpenAI TTS max is 4096 chars, leave some margin
)

// Service handles text-to-speech conversion using OpenAI's API
type Service struct {
	apiKey   string
	model    string
	voice    string
	audioDir string
	client   *http.Client
}

// New creates a new TTS service
func New(apiKey, model, voice, audioDir string) (*Service, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is required for TTS")
	}

	// Ensure audio directory exists
	if err := os.MkdirAll(audioDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create audio directory %s: %w", audioDir, err)
	}

	return &Service{
		apiKey:   apiKey,
		model:    model,
		voice:    voice,
		audioDir: audioDir,
		client:   &http.Client{},
	}, nil
}

// ttsRequest is the request body for OpenAI's TTS API
type ttsRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
	Voice string `json:"voice"`
}

// GenerateAudio converts article text to an MP3 file.
// Returns the file path and file size.
func (s *Service) GenerateAudio(ctx context.Context, articleID int, text, voice string) (string, int64, error) {
	if voice == "" {
		voice = s.voice
	}

	// Clean up text for TTS
	text = cleanTextForTTS(text)
	if text == "" {
		return "", 0, fmt.Errorf("no text content to convert")
	}

	// Chunk the text if it exceeds the max size
	chunks := chunkText(text, maxChunkSize)
	log.Printf("TTS: article %d - %d characters, %d chunk(s), voice=%s", articleID, len(text), len(chunks), voice)

	// Generate audio for each chunk
	var audioData bytes.Buffer
	for i, chunk := range chunks {
		log.Printf("TTS: article %d - generating chunk %d/%d", articleID, i+1, len(chunks))

		data, err := s.callTTSAPI(ctx, chunk, voice)
		if err != nil {
			return "", 0, fmt.Errorf("failed to generate audio for chunk %d: %w", i+1, err)
		}
		audioData.Write(data)
	}

	// Write the combined audio to a file
	filename := fmt.Sprintf("%d_%s.mp3", articleID, voice)
	filePath := filepath.Join(s.audioDir, filename)

	if err := os.WriteFile(filePath, audioData.Bytes(), 0644); err != nil {
		return "", 0, fmt.Errorf("failed to write audio file: %w", err)
	}

	fileSize := int64(audioData.Len())
	log.Printf("TTS: article %d - audio saved to %s (%d bytes)", articleID, filePath, fileSize)

	return filePath, fileSize, nil
}

// GetAudioPath returns the expected file path for an article's audio
func (s *Service) GetAudioPath(articleID int, voice string) string {
	if voice == "" {
		voice = s.voice
	}
	filename := fmt.Sprintf("%d_%s.mp3", articleID, voice)
	return filepath.Join(s.audioDir, filename)
}

// AudioDir returns the audio storage directory
func (s *Service) AudioDir() string {
	return s.audioDir
}

// AvailableVoices returns the list of available OpenAI TTS voices
func AvailableVoices() []string {
	return []string{"alloy", "echo", "fable", "onyx", "nova", "shimmer"}
}

// callTTSAPI makes a single request to OpenAI's TTS API
func (s *Service) callTTSAPI(ctx context.Context, text, voice string) ([]byte, error) {
	reqBody := ttsRequest{
		Model: s.model,
		Input: text,
		Voice: voice,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAITTSURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("TTS API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TTS API returned status %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// cleanTextForTTS prepares text for TTS conversion
func cleanTextForTTS(text string) string {
	// Remove excessive whitespace
	text = strings.TrimSpace(text)

	// Replace multiple newlines with a single pause marker
	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}

	return strings.Join(cleaned, "\n")
}

// chunkText splits text into chunks that fit within the TTS API's character limit,
// breaking at sentence boundaries when possible
func chunkText(text string, maxSize int) []string {
	if len(text) <= maxSize {
		return []string{text}
	}

	var chunks []string
	remaining := text

	for len(remaining) > 0 {
		if len(remaining) <= maxSize {
			chunks = append(chunks, remaining)
			break
		}

		// Find the best break point within the limit
		chunk := remaining[:maxSize]
		breakPoint := findBreakPoint(chunk)

		chunks = append(chunks, strings.TrimSpace(remaining[:breakPoint]))
		remaining = strings.TrimSpace(remaining[breakPoint:])
	}

	return chunks
}

// findBreakPoint finds the best position to break text, preferring sentence endings
func findBreakPoint(text string) int {
	// Try to break at sentence endings (. ! ?) followed by space
	for i := len(text) - 1; i > len(text)/2; i-- {
		if i+1 < len(text) && (text[i] == '.' || text[i] == '!' || text[i] == '?') && unicode.IsSpace(rune(text[i+1])) {
			return i + 1
		}
	}

	// Fall back to breaking at newline
	for i := len(text) - 1; i > len(text)/2; i-- {
		if text[i] == '\n' {
			return i + 1
		}
	}

	// Fall back to breaking at space
	for i := len(text) - 1; i > len(text)/2; i-- {
		if unicode.IsSpace(rune(text[i])) {
			return i + 1
		}
	}

	// Last resort: break at the limit
	return len(text)
}
