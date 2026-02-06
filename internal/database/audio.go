package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// CreateAudioFile inserts a new audio file record
func (db *DB) CreateAudioFile(ctx context.Context, audio *AudioFile) error {
	query := `
		INSERT INTO audio_files (article_id, voice, file_path, file_size, status, error_message)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (article_id, voice) DO UPDATE SET
			file_path = EXCLUDED.file_path,
			file_size = EXCLUDED.file_size,
			status = EXCLUDED.status,
			error_message = EXCLUDED.error_message
		RETURNING id, created_at, updated_at
	`

	err := db.pool.QueryRow(ctx, query,
		audio.ArticleID,
		audio.Voice,
		audio.FilePath,
		audio.FileSize,
		audio.Status,
		audio.ErrorMessage,
	).Scan(&audio.ID, &audio.CreatedAt, &audio.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create audio file: %w", err)
	}

	return nil
}

// UpdateAudioFileStatus updates the status and optional error message of an audio file
func (db *DB) UpdateAudioFileStatus(ctx context.Context, id int, status string, filePath string, fileSize int64, errorMsg *string) error {
	query := `
		UPDATE audio_files
		SET status = $1, file_path = $2, file_size = $3, error_message = $4
		WHERE id = $5
	`

	_, err := db.pool.Exec(ctx, query, status, filePath, fileSize, errorMsg, id)
	if err != nil {
		return fmt.Errorf("failed to update audio file status: %w", err)
	}

	return nil
}

// GetAudioFileByArticle retrieves an audio file for a given article and voice
func (db *DB) GetAudioFileByArticle(ctx context.Context, articleID int, voice string) (*AudioFile, error) {
	query := `
		SELECT id, article_id, voice, file_path, file_size, status, error_message, created_at, updated_at
		FROM audio_files
		WHERE article_id = $1 AND voice = $2
	`

	var audio AudioFile
	err := db.pool.QueryRow(ctx, query, articleID, voice).Scan(
		&audio.ID,
		&audio.ArticleID,
		&audio.Voice,
		&audio.FilePath,
		&audio.FileSize,
		&audio.Status,
		&audio.ErrorMessage,
		&audio.CreatedAt,
		&audio.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get audio file: %w", err)
	}

	return &audio, nil
}

// GetAudioFilesByArticle retrieves all audio files for a given article
func (db *DB) GetAudioFilesByArticle(ctx context.Context, articleID int) ([]*AudioFile, error) {
	query := `
		SELECT id, article_id, voice, file_path, file_size, status, error_message, created_at, updated_at
		FROM audio_files
		WHERE article_id = $1
		ORDER BY created_at DESC
	`

	rows, err := db.pool.Query(ctx, query, articleID)
	if err != nil {
		return nil, fmt.Errorf("failed to query audio files: %w", err)
	}
	defer rows.Close()

	var audioFiles []*AudioFile
	for rows.Next() {
		var audio AudioFile
		err := rows.Scan(
			&audio.ID,
			&audio.ArticleID,
			&audio.Voice,
			&audio.FilePath,
			&audio.FileSize,
			&audio.Status,
			&audio.ErrorMessage,
			&audio.CreatedAt,
			&audio.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audio file: %w", err)
		}
		audioFiles = append(audioFiles, &audio)
	}

	return audioFiles, nil
}

// DeleteAudioFile deletes an audio file record by ID
func (db *DB) DeleteAudioFile(ctx context.Context, id int) error {
	query := `DELETE FROM audio_files WHERE id = $1`

	result, err := db.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete audio file: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("audio file not found")
	}

	return nil
}

// GetCompletedAudioForArticles retrieves completed audio files for a list of article IDs.
// Returns a map of articleID -> AudioFile for quick lookups.
func (db *DB) GetCompletedAudioForArticles(ctx context.Context, articleIDs []int) (map[int]*AudioFile, error) {
	if len(articleIDs) == 0 {
		return make(map[int]*AudioFile), nil
	}

	query := `
		SELECT DISTINCT ON (article_id)
			id, article_id, voice, file_path, file_size, status, error_message, created_at, updated_at
		FROM audio_files
		WHERE article_id = ANY($1) AND status = 'completed'
		ORDER BY article_id, created_at DESC
	`

	rows, err := db.pool.Query(ctx, query, articleIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to query audio files for articles: %w", err)
	}
	defer rows.Close()

	result := make(map[int]*AudioFile)
	for rows.Next() {
		var audio AudioFile
		err := rows.Scan(
			&audio.ID,
			&audio.ArticleID,
			&audio.Voice,
			&audio.FilePath,
			&audio.FileSize,
			&audio.Status,
			&audio.ErrorMessage,
			&audio.CreatedAt,
			&audio.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audio file: %w", err)
		}
		result[audio.ArticleID] = &audio
	}

	return result, nil
}
