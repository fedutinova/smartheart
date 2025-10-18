# EKG Analysis API Usage

## Overview
The SmartHeart system now supports EKG image analysis with advanced preprocessing using OpenCV.

## API Endpoints

### Submit EKG Analysis
```http
POST /v1/ekg/analyze
Authorization: Bearer <access_token>
Content-Type: application/json

{
  "image_temp_url": "https://example.com/ekg-image.jpg",
  "notes": "Patient EKG from emergency room"
}
```

**Response:**
```json
{
  "job_id": "123e4567-e89b-12d3-a456-426614174000",
  "status": "queued",
  "message": "EKG analysis job submitted successfully"
}
```

### Check Job Status
```http
GET /v1/jobs/{job_id}
Authorization: Bearer <access_token>
```

**Response:**
```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "type": "ekg_analyze",
  "status": "succeeded",
  "enqueued_at": "2024-01-01T12:00:00Z",
  "started_at": "2024-01-01T12:00:01Z",
  "finished_at": "2024-01-01T12:00:05Z"
}
```

### Get Analysis Results
```http
GET /v1/requests/{request_id}
Authorization: Bearer <access_token>
```

**Response:**
```json
{
  "id": "456e7890-e89b-12d3-a456-426614174001",
  "user_id": "789e0123-e89b-12d3-a456-426614174002",
  "text_query": "Patient EKG from emergency room",
  "status": "completed",
  "created_at": "2024-01-01T12:00:00Z",
  "updated_at": "2024-01-01T12:00:05Z",
  "response": {
    "id": "abc12345-e89b-12d3-a456-426614174003",
    "request_id": "456e7890-e89b-12d3-a456-426614174001",
    "content": "{\"analysis_type\":\"ekg_preprocessing\",\"signal_length\":150.5,\"signal_features\":{\"points_count\":1250,\"signal_width\":800,\"amplitude_range\":120,\"baseline\":300.5,\"standard_deviation\":25.3,\"bounding_box\":{\"min_x\":0,\"max_x\":800,\"min_y\":240,\"max_y\":360}},\"processing_steps\":[\"resized\",\"grayscale\",\"contrast_enhanced\",\"binarized\",\"morphological_processed\",\"signal_extracted\"],\"contour_points\":1250,\"notes\":\"Patient EKG from emergency room\",\"timestamp\":\"2024-01-01T12:00:05Z\",\"job_id\":\"123e4567-e89b-12d3-a456-426614174000\"}",
    "model": "ekg_preprocessor_v1",
    "tokens_used": 0,
    "processing_time_ms": 0,
    "created_at": "2024-01-01T12:00:05Z"
  }
}
```

## EKG Processing Pipeline

The system performs the following preprocessing steps:

1. **Image Resize**: Resizes to fixed dimensions (800x600)
2. **Grayscale Conversion**: Converts color image to grayscale
3. **Contrast Enhancement**: Applies histogram equalization
4. **Binarization**: Uses adaptive threshold for binary conversion
5. **Morphological Operations**: Applies erosion and dilation for noise reduction
6. **Signal Extraction**: Finds the longest contour as the EKG signal

## Signal Features

The analysis extracts the following features:

- **Signal Length**: Arc length of the detected contour
- **Signal Width**: Horizontal span of the signal
- **Amplitude Range**: Vertical range of signal variation
- **Baseline**: Average Y position of the signal
- **Standard Deviation**: Signal variation from baseline
- **Bounding Box**: Min/max coordinates of the signal
- **Contour Points**: Number of points in the detected contour

## Supported Image Formats

- JPEG/JPG
- PNG
- GIF
- WebP
- BMP
- TIFF
- PDF (containing images)

## Limits

- Maximum file size: 10MB
- Download timeout: 30 seconds
- Processing timeout: 30 seconds

## Error Handling

The system handles various error scenarios:

- Invalid image URL
- Download failures
- Unsupported image formats
- Processing errors
- Timeout errors

All errors are logged with detailed information for debugging.
