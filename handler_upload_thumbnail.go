package main

import (
	"net/http"
	"database/sql"
	"os"
	"io"
	"mime"
	"crypto/rand"
	"encoding/base64"

	"github.com/google/uuid"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	const maxMemory = 10 << 20
	r.ParseMultipartForm(maxMemory)
	file, fileHeader, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	mediaType, _, err := mime.ParseMediaType(fileHeader.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Missing Content-Type for thumbnail", nil)
		return
	}
	if mediaType != "image/png" && mediaType != "image/jpeg"{
		respondWithError(w, http.StatusBadRequest, "Invalid mime type", nil)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err == sql.ErrNoRows {
		respondWithError(w, http.StatusNotFound, "error getting the video", err)
		return
	}
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting the video", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "error not your video", err)
		return
	}

	key := make([]byte, 32)
	rand.Read(key)
	thumbnailID := base64.RawURLEncoding.EncodeToString(key)
	filename := getFilename(thumbnailID, mediaType)
	thumbnailDiskPath := cfg.getAssetDiskPath(filename)

	dst, err := os.Create(thumbnailDiskPath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error creating thumbnail", err)
		return
	}
	defer dst.Close()

	_, err = io.Copy(dst, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error copying thumbnail", err)
		return
	}

	thumbnailURL := cfg.getAssetURL(filename)
	video.ThumbnailURL = &thumbnailURL
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error updating video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
