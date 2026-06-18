package main

import (
	"net/http"
	"os"
	"io"
	"mime"
	"fmt"

	"github.com/314159otr/Tubely/internal/auth"

	"github.com/google/uuid"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1 << 30)
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

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error getting video", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "User is not the owner of this video", err)
		return
	}

	file, fileHeader, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "no video file attached", err)
		return
	}
	defer file.Close()
	mediaType, _, err := mime.ParseMediaType(fileHeader.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Missing Content-Type for video", nil)
		return
	}
	if mediaType != "video/mp4"{
		respondWithError(w, http.StatusBadRequest, "Invalid mime type", nil)
		return
	}
	assetName := getAssetRandomName()
	filename := getFilename(assetName, mediaType)

	tempFile, err := os.CreateTemp("", filename)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error creating temp file", nil)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()
	if _, err := io.Copy(tempFile, file); err != nil {
		respondWithError(w, http.StatusInternalServerError, "error copying to temp file", nil)
		return
	}

	aspectRatio, err :=  getVideoAspectRatio(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not get video aspect ratio", err)
		return
	}
	var prefix string
	if aspectRatio == "16:9" {
		prefix = "landscape/"
	} else if aspectRatio == "9:16" {
		prefix = "portrait/"
	} else {
		prefix = "other/"
	}
	filename = prefix + filename

	_, err = tempFile.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not reset temp file pointer", err)
		return
	}

	processedTempFilePath, err := processVideoForFastStart(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not process video for fast start", err)
		return
	}
	processedTempFile, err := os.Open(processedTempFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not open processed video", err)
		return
	}
	defer processedTempFile.Close()

	putObjectInput := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &filename,
		Body:        processedTempFile,
		ContentType: &mediaType,
	}
	if _, err := cfg.s3Client.PutObject(r.Context(), &putObjectInput); err != nil {
		respondWithError(w, http.StatusInternalServerError, "error uploading file to AWS", nil)
		return
	}

	videoURL := cfg.getAssetCloudfrontURL(filename)
	fmt.Println(videoURL)
	video.VideoURL = &videoURL
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "error updating video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}
