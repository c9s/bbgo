package types

import "bytes"

type FileType string

const (
	FileTypeImage    FileType = "image"
	FileTypeVideo    FileType = "video"
	FileTypeAudio    FileType = "audio"
	FileTypeDocument FileType = "document"
	FileTypeText     FileType = "text"
)

type UploadFile struct {
	Caption  string
	FileType FileType
	FileName string
	Data     *bytes.Buffer

	// References is a map of additional metadata that can be used to store
	// For Slack upload, this can be used to store the uploaded file object
	References map[string]any
}

func (f *UploadFile) SetReference(key string, value any) {
	if f.References == nil {
		f.References = make(map[string]any)
	}

	f.References[key] = value
}
