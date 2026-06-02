package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/fedutinova/smartheart/back-api/cv"
	"github.com/fedutinova/smartheart/back-api/models"
	"github.com/fedutinova/smartheart/back-api/service"
)

// SubmitECGAnalyze handles EKG image analysis submission as a multipart/form-data
// upload. URL-based submission was removed: a URL-fetched image cannot carry the
// client-side OCR redaction that file uploads do, so multipart upload of a
// pre-redacted image is the only supported ingest path.
func (h *ECGHandler) SubmitECGAnalyze(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse form")
		return
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "image file is required")
		return
	}
	defer func() { _ = file.Close() }()

	params := service.ECGParams{
		Sex:            r.FormValue("sex"),
		PaperSpeedMMS:  25,
		MmPerMvLimb:    10,
		MmPerMvChest:   10,
		LayoutLabel:    cv.DefaultLayout,
		PreprocessName: cv.DefaultPreprocess,
	}
	if v := r.FormValue("layout_label"); v != "" {
		if !cv.IsValidLayout(v) {
			writeError(w, http.StatusBadRequest, "invalid layout_label")
			return
		}
		params.LayoutLabel = v
	}
	if v := r.FormValue("preprocess_name"); v != "" {
		if !cv.IsValidPreprocess(v) {
			writeError(w, http.StatusBadRequest, "invalid preprocess_name")
			return
		}
		params.PreprocessName = v
	}
	if rawClientMeta := r.FormValue("client_meta"); rawClientMeta != "" {
		var clientMeta models.RequestClientMeta
		if err := json.Unmarshal([]byte(rawClientMeta), &clientMeta); err != nil {
			writeError(w, http.StatusBadRequest, "invalid client_meta")
			return
		}
		if err := clientMeta.Validate(); err != nil {
			writeError(w, http.StatusBadRequest, "invalid client_meta")
			return
		}
		params.ClientMeta = &clientMeta
	}
	if v := r.FormValue("age"); v != "" {
		if age, err := strconv.Atoi(v); err == nil && age > 0 && age <= 150 {
			params.Age = &age
		}
	}
	if v := r.FormValue("paper_speed_mms"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 10 && f <= 100 {
			params.PaperSpeedMMS = f
		}
	}
	if v := r.FormValue("mm_per_mv_limb"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 1 && f <= 40 {
			params.MmPerMvLimb = f
		}
	}
	if v := r.FormValue("mm_per_mv_chest"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 1 && f <= 40 {
			params.MmPerMvChest = f
		}
	}

	userID, _, ok := extractUserID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	uploaded := service.UploadedFile{
		Reader:      file,
		Filename:    header.Filename,
		ContentType: header.Header.Get("Content-Type"),
		Size:        header.Size,
	}

	result, err := h.Service.SubmitECGFile(r.Context(), userID, uploaded, params)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, SubmitECGResponse{
		JobID:     result.JobID,
		RequestID: result.RequestID,
		Status:    result.Status,
		Message:   fmt.Sprintf("EKG analysis job submitted successfully (file: %s)", header.Filename),
	})
}
