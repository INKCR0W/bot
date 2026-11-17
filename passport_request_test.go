package bot

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/go-telegram/bot/models"
)

func TestSetPassportDataErrorsEncodesSources(t *testing.T) {
	errors := []models.PassportElementError{
		&models.PassportElementErrorDataField{Type: "personal_details", FieldName: "first_name", DataHash: "data-hash", Message: "check name"},
		&models.PassportElementErrorFrontSide{Type: "passport", FileHash: "front-hash", Message: "check front"},
		&models.PassportElementErrorReverseSide{Type: "identity_card", FileHash: "reverse-hash", Message: "check reverse"},
		&models.PassportElementErrorSelfie{Type: "passport", FileHash: "selfie-hash", Message: "check selfie"},
		&models.PassportElementErrorFile{Type: "utility_bill", FileHash: "file-hash", Message: "check bill"},
		&models.PassportElementErrorFiles{Type: "bank_statement", FileHashes: []string{"file-hash-1", "file-hash-2"}, Message: "check scans"},
		&models.PassportElementErrorTranslationFile{Type: "driver_license", FileHash: "translation-hash", Message: "check translation"},
		&models.PassportElementErrorTranslationFiles{Type: "rental_agreement", FileHashes: []string{"translation-hash-1", "translation-hash-2"}, Message: "check translations"},
		&models.PassportElementErrorUnspecified{Type: "address", ElementHash: "element-hash", Message: "check address"},
	}
	c := &httpClient{
		t:       t,
		reqPath: "/bot/setPassportDataErrors",
		resp:    `true`,
		reqFields: map[string]string{
			"user_id": "123",
			"errors": `[` +
				`{"source":"data","type":"personal_details","field_name":"first_name","data_hash":"data-hash","message":"check name"},` +
				`{"source":"front_side","type":"passport","file_hash":"front-hash","message":"check front"},` +
				`{"source":"reverse_side","type":"identity_card","file_hash":"reverse-hash","message":"check reverse"},` +
				`{"source":"selfie","type":"passport","file_hash":"selfie-hash","message":"check selfie"},` +
				`{"source":"file","type":"utility_bill","file_hash":"file-hash","message":"check bill"},` +
				`{"source":"files","type":"bank_statement","file_hashes":["file-hash-1","file-hash-2"],"message":"check scans"},` +
				`{"source":"translation_file","type":"driver_license","file_hash":"translation-hash","message":"check translation"},` +
				`{"source":"translation_files","type":"rental_agreement","file_hashes":["translation-hash-1","translation-hash-2"],"message":"check translations"},` +
				`{"source":"unspecified","type":"address","element_hash":"element-hash","message":"check address"}]`,
		},
	}
	b := &Bot{client: c}
	ok, err := b.SetPassportDataErrors(context.Background(), &SetPassportDataErrorsParams{UserID: 123, Errors: errors})
	if err != nil || !ok {
		t.Fatalf("got result %v, error %v", ok, err)
	}
	for _, element := range errors {
		standard, err := json.Marshal(element)
		if err != nil {
			t.Fatal(err)
		}
		custom, err := element.MarshalCustom()
		if err != nil || !bytes.Equal(custom, standard) {
			t.Errorf("%T MarshalCustom returned %s, %v, want %s", element, custom, err, standard)
		}
	}
}
