package models

import (
	"encoding/json"
)

// PassportElementError https://core.telegram.org/bots/api#passportelementerror
type PassportElementError interface {
	passportElementErrorTag()

	MarshalCustom() ([]byte, error)
}

// PassportElementErrorDataField https://core.telegram.org/bots/api#passportelementerrordatafield
type PassportElementErrorDataField struct {
	Type      string `json:"type"`
	FieldName string `json:"field_name"`
	DataHash  string `json:"data_hash"`
	Message   string `json:"message"`
}

func (PassportElementErrorDataField) passportElementErrorTag() {}

func (m PassportElementErrorDataField) MarshalJSON() ([]byte, error) {
	type payload PassportElementErrorDataField
	return json.Marshal(struct {
		Source string `json:"source"`
		payload
	}{Source: "data", payload: payload(m)})
}

func (m *PassportElementErrorDataField) MarshalCustom() ([]byte, error) {
	return json.Marshal(m)
}

// PassportElementErrorFrontSide https://core.telegram.org/bots/api#passportelementerrorfrontside
type PassportElementErrorFrontSide struct {
	Type     string `json:"type"`
	FileHash string `json:"file_hash"`
	Message  string `json:"message"`
}

func (PassportElementErrorFrontSide) passportElementErrorTag() {}

func (m PassportElementErrorFrontSide) MarshalJSON() ([]byte, error) {
	type payload PassportElementErrorFrontSide
	return json.Marshal(struct {
		Source string `json:"source"`
		payload
	}{Source: "front_side", payload: payload(m)})
}

func (m *PassportElementErrorFrontSide) MarshalCustom() ([]byte, error) {
	return json.Marshal(m)
}

// PassportElementErrorReverseSide https://core.telegram.org/bots/api#passportelementerrorreverseside
type PassportElementErrorReverseSide struct {
	Type     string `json:"type"`
	FileHash string `json:"file_hash"`
	Message  string `json:"message"`
}

func (PassportElementErrorReverseSide) passportElementErrorTag() {}

func (m PassportElementErrorReverseSide) MarshalJSON() ([]byte, error) {
	type payload PassportElementErrorReverseSide
	return json.Marshal(struct {
		Source string `json:"source"`
		payload
	}{Source: "reverse_side", payload: payload(m)})
}

func (m *PassportElementErrorReverseSide) MarshalCustom() ([]byte, error) {
	return json.Marshal(m)
}

// PassportElementErrorSelfie https://core.telegram.org/bots/api#passportelementerrorselfie
type PassportElementErrorSelfie struct {
	Type     string `json:"type"`
	FileHash string `json:"file_hash"`
	Message  string `json:"message"`
}

func (PassportElementErrorSelfie) passportElementErrorTag() {}

func (m PassportElementErrorSelfie) MarshalJSON() ([]byte, error) {
	type payload PassportElementErrorSelfie
	return json.Marshal(struct {
		Source string `json:"source"`
		payload
	}{Source: "selfie", payload: payload(m)})
}

func (m *PassportElementErrorSelfie) MarshalCustom() ([]byte, error) {
	return json.Marshal(m)
}

// PassportElementErrorFile https://core.telegram.org/bots/api#passportelementerrorfile
type PassportElementErrorFile struct {
	Type     string `json:"type"`
	FileHash string `json:"file_hash"`
	Message  string `json:"message"`
}

func (PassportElementErrorFile) passportElementErrorTag() {}

func (m PassportElementErrorFile) MarshalJSON() ([]byte, error) {
	type payload PassportElementErrorFile
	return json.Marshal(struct {
		Source string `json:"source"`
		payload
	}{Source: "file", payload: payload(m)})
}

func (m *PassportElementErrorFile) MarshalCustom() ([]byte, error) {
	return json.Marshal(m)
}

// PassportElementErrorFiles https://core.telegram.org/bots/api#passportelementerrorfiles
type PassportElementErrorFiles struct {
	Type       string   `json:"type"`
	FileHashes []string `json:"file_hashes"`
	Message    string   `json:"message"`
}

func (PassportElementErrorFiles) passportElementErrorTag() {}

func (m PassportElementErrorFiles) MarshalJSON() ([]byte, error) {
	type payload PassportElementErrorFiles
	return json.Marshal(struct {
		Source string `json:"source"`
		payload
	}{Source: "files", payload: payload(m)})
}

func (m *PassportElementErrorFiles) MarshalCustom() ([]byte, error) {
	return json.Marshal(m)
}

// PassportElementErrorTranslationFile https://core.telegram.org/bots/api#passportelementerrortranslationfile
type PassportElementErrorTranslationFile struct {
	Type     string `json:"type"`
	FileHash string `json:"file_hash"`
	Message  string `json:"message"`
}

func (PassportElementErrorTranslationFile) passportElementErrorTag() {}

func (m PassportElementErrorTranslationFile) MarshalJSON() ([]byte, error) {
	type payload PassportElementErrorTranslationFile
	return json.Marshal(struct {
		Source string `json:"source"`
		payload
	}{Source: "translation_file", payload: payload(m)})
}

func (m *PassportElementErrorTranslationFile) MarshalCustom() ([]byte, error) {
	return json.Marshal(m)
}

// PassportElementErrorTranslationFiles https://core.telegram.org/bots/api#passportelementerrortranslationfiles
type PassportElementErrorTranslationFiles struct {
	Type       string   `json:"type"`
	FileHashes []string `json:"file_hashes"`
	Message    string   `json:"message"`
}

func (PassportElementErrorTranslationFiles) passportElementErrorTag() {}

func (m PassportElementErrorTranslationFiles) MarshalJSON() ([]byte, error) {
	type payload PassportElementErrorTranslationFiles
	return json.Marshal(struct {
		Source string `json:"source"`
		payload
	}{Source: "translation_files", payload: payload(m)})
}

func (m *PassportElementErrorTranslationFiles) MarshalCustom() ([]byte, error) {
	return json.Marshal(m)
}

// PassportElementErrorUnspecified https://core.telegram.org/bots/api#passportelementerrorunspecified
type PassportElementErrorUnspecified struct {
	Type        string `json:"type"`
	ElementHash string `json:"element_hash"`
	Message     string `json:"message"`
}

func (PassportElementErrorUnspecified) passportElementErrorTag() {}

func (m PassportElementErrorUnspecified) MarshalJSON() ([]byte, error) {
	type payload PassportElementErrorUnspecified
	return json.Marshal(struct {
		Source string `json:"source"`
		payload
	}{Source: "unspecified", payload: payload(m)})
}

func (m *PassportElementErrorUnspecified) MarshalCustom() ([]byte, error) {
	return json.Marshal(m)
}
