package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/fedutinova/smartheart/back-api/service"
)

type ekgAnalyzeRequest struct {
	ImageTempURL  string   `json:"image_temp_url"            validate:"required,url"`
	Age           *int     `json:"age,omitempty"             validate:"omitempty,min=1,max=150"`
	Sex           string   `json:"sex,omitempty"             validate:"omitempty,oneof=male female"`
	PaperSpeedMMS *float64 `json:"paper_speed_mms,omitempty" validate:"omitempty,min=10,max=100"`
	MmPerMvLimb   *float64 `json:"mm_per_mv_limb,omitempty"  validate:"omitempty,min=1,max=40"`
	MmPerMvChest  *float64 `json:"mm_per_mv_chest,omitempty" validate:"omitempty,min=1,max=40"`
}

// SubmitEKGAnalyze handles EKG image analysis submission.
// Accepts either JSON (URL mode) or multipart/form-data (file upload mode).
func (h *EKGHandler) SubmitEKGAnalyze(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")

	if strings.HasPrefix(contentType, "multipart/form-data") {
		h.submitEKGFile(w, r)
		return
	}

	h.submitEKGURL(w, r)
}

func ecgParamsFromRequest(req *ekgAnalyzeRequest) service.ECGParams {
	p := service.ECGParams{
		Age:           req.Age,
		Sex:           req.Sex,
		PaperSpeedMMS: 25,
		MmPerMvLimb:   10,
		MmPerMvChest:  10,
	}
	if req.PaperSpeedMMS != nil {
		p.PaperSpeedMMS = *req.PaperSpeedMMS
	}
	if req.MmPerMvLimb != nil {
		p.MmPerMvLimb = *req.MmPerMvLimb
	}
	if req.MmPerMvChest != nil {
		p.MmPerMvChest = *req.MmPerMvChest
	}
	return p
}

// submitEKGURL handles URL-based EKG submission.
func (h *EKGHandler) submitEKGURL(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	var req ekgAnalyzeRequest
	if !decodeAndValidate(w, r, &req) {
		return
	}

	userID, _, ok := extractUserID(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	params := ecgParamsFromRequest(&req)
	result, err := h.Service.SubmitEKG(r.Context(), userID, req.ImageTempURL, params)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, SubmitEKGResponse{
		JobID:     result.JobID,
		RequestID: result.RequestID,
		Status:    result.Status,
		Message:   "EKG analysis job submitted successfully",
	})
}

// submitEKGFile handles file-based EKG submission (multipart upload).
func (h *EKGHandler) submitEKGFile(w http.ResponseWriter, r *http.Request) {
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
		Sex:           r.FormValue("sex"),
		PaperSpeedMMS: 25,
		MmPerMvLimb:   10,
		MmPerMvChest:  10,
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

	result, err := h.Service.SubmitEKGFile(r.Context(), userID, uploaded, params)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, SubmitEKGResponse{
		JobID:     result.JobID,
		RequestID: result.RequestID,
		Status:    result.Status,
		Message:   fmt.Sprintf("EKG analysis job submitted successfully (file: %s)", header.Filename),
	})
}
