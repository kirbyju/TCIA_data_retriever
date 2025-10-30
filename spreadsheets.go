package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tealeg/xlsx"
)

// SpreadSheetDecoder is an interface for decoding spreadsheets
type SpreadSheetDecoder interface {
	Decode(file *os.File) ([][]string, error)
}

// CSVDecoder decodes CSV files
type CSVDecoder struct{}

// TSVDecoder decodes TSV files
type TSVDecoder struct{}

// XLSXDecoder decodes XLSX files
type XLSXDecoder struct{}

// Decode decodes a CSV file and returns the values from the "imageUrl" or "drs_uri" column
func (d *CSVDecoder) Decode(file *os.File) ([][]string, error) {
	return decodesv(file, ',')
}

// Decode decodes a TSV file and returns the values from the "imageUrl" or "drs_uri" column
func (d *TSVDecoder) Decode(file *os.File) ([][]string, error) {
	return decodesv(file, '\t')
}

// decodesv decodes a separated value file and returns the values from the "imageUrl" or "drs_uri" column
func decodesv(file *os.File, separator rune) ([][]string, error) {
	reader := csv.NewReader(file)
	reader.Comma = separator
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	return records, nil
}

// Decode decodes an XLSX file and returns the values from the "imageUrl" or "drs_uri" column
func (d *XLSXDecoder) Decode(file *os.File) ([][]string, error) {
	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("could not get file stats: %w", err)
	}
	size := stat.Size()
	xlFile, err := xlsx.OpenReaderAt(file, size)
	if err != nil {
		return nil, err
	}

	var records [][]string
	for _, sheet := range xlFile.Sheets {
		for _, row := range sheet.Rows {
			var record []string
			for _, cell := range row.Cells {
				record = append(record, cell.String())
			}
			records = append(records, record)
		}
	}
	return records, nil
}

// getSpreadsheetDecoder returns a decoder based on the file extension
func getSpreadsheetDecoder(filename string) (SpreadSheetDecoder, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".csv":
		return &CSVDecoder{}, nil
	case ".tsv":
		return &TSVDecoder{}, nil
	case ".xlsx":
		return &XLSXDecoder{}, nil
	default:
		return nil, fmt.Errorf("unsupported spreadsheet format: %s", ext)
	}
}

// decodeSpreadsheet decodes a spreadsheet file and returns a slice of FileInfo objects
func decodeSpreadsheet(filePath string) ([]*FileInfo, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder, err := getSpreadsheetDecoder(filePath)
	if err != nil {
		return nil, err
	}

	records, err := decoder.Decode(file)
	if err != nil {
		return nil, err
	}

	if len(records) == 0 {
		return []*FileInfo{}, nil
	}

	header := records[0]
	drsURIIndex := -1
	imageURLIndex := -1
	for i, col := range header {
		switch col {
		case "drs_uri":
			drsURIIndex = i
		case "imageUrl":
			imageURLIndex = i
		}
	}

	if drsURIIndex == -1 && imageURLIndex == -1 {
		return nil, fmt.Errorf("no 'drs_uri' or 'imageUrl' column found in %s", file.Name())
	}

	var fileInfos []*FileInfo
	for _, record := range records[1:] {
		if drsURIIndex != -1 {
			if len(record) > drsURIIndex {
				uri := record[drsURIIndex]
				fileInfos = append(fileInfos, &FileInfo{
					DRSURI:    uri,
					SeriesUID: filepath.Base(uri),
				})
			}
		} else {
			if len(record) > imageURLIndex {
				url := record[imageURLIndex]
				fileInfos = append(fileInfos, &FileInfo{
					DownloadURL: url,
					SeriesUID:   filepath.Base(url),
				})
			}
		}
	}

	return fileInfos, nil
}