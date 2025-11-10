package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/suyashkumar/dicom"
	"github.com/suyashkumar/dicom/pkg/tag"
)

type DicomFile struct {
	Path             string
	SeriesUID        string
	AcquisitionNumber int
	InstanceNumber   int
}

func ProcessDicomFile(filePath string) (*DicomFile, error) {
	dataset, err := dicom.ParseFile(filePath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DICOM file %s: %v", filePath, err)
	}

	seriesUID, err := getElementValue(dataset, tag.SeriesInstanceUID)
	if err != nil {
		return nil, err
	}
	acquisitionNumberStr, err := getElementValue(dataset, tag.AcquisitionNumber)
	if err != nil {
		// AcquisitionNumber is optional, so we can default to 0
		acquisitionNumberStr = "0"
	}
	instanceNumberStr, err := getElementValue(dataset, tag.InstanceNumber)
	if err != nil {
		return nil, err
	}

	acquisitionNumber, _ := strconv.Atoi(acquisitionNumberStr)
	instanceNumber, _ := strconv.Atoi(instanceNumberStr)

	return &DicomFile{
		Path:             filePath,
		SeriesUID:        seriesUID,
		AcquisitionNumber: acquisitionNumber,
		InstanceNumber:   instanceNumber,
	}, nil
}

func getElementValue(dataset dicom.Dataset, t tag.Tag) (string, error) {
	element, err := dataset.FindElementByTag(t)
	if err != nil {
		return "", fmt.Errorf("could not find tag %v in DICOM", t)
	}
	// The dicom library sometimes returns values with brackets, so we trim them.
	return strings.Trim(element.Value.String(), "[]"), nil
}

func OrganizeDicomFiles(files []*DicomFile, outputDir string) error {
	series := make(map[string][]*DicomFile)
	for _, file := range files {
		series[file.SeriesUID] = append(series[file.SeriesUID], file)
	}

	for seriesUID, dicomFiles := range series {
		seriesDir := filepath.Join(outputDir, seriesUID)
		if err := os.MkdirAll(seriesDir, 0755); err != nil {
			return fmt.Errorf("could not create series directory %s: %v", seriesDir, err)
		}

		// Sort files by AcquisitionNumber, then InstanceNumber
		sort.Slice(dicomFiles, func(i, j int) bool {
			if dicomFiles[i].AcquisitionNumber != dicomFiles[j].AcquisitionNumber {
				return dicomFiles[i].AcquisitionNumber < dicomFiles[j].AcquisitionNumber
			}
			return dicomFiles[i].InstanceNumber < dicomFiles[j].InstanceNumber
		})

		// Map original acquisition numbers to new ordinal ones starting from 1.
		// Empty acquisition number is treated as the lowest.
		acquisitionMap := make(map[int]int)
		ordinalAcquisitionNum := 1
		if len(dicomFiles) > 0 {
			// Create a map to store unique acquisition numbers.
			uniqueAcquisitions := make(map[int]bool)
			for _, file := range dicomFiles {
				uniqueAcquisitions[file.AcquisitionNumber] = true
			}
			// Extract and sort the unique acquisition numbers.
			sortedAcquisitions := make([]int, 0, len(uniqueAcquisitions))
			for acq := range uniqueAcquisitions {
				sortedAcquisitions = append(sortedAcquisitions, acq)
			}
			sort.Ints(sortedAcquisitions)
			// Create the map from original to ordinal acquisition number.
			for _, acqNum := range sortedAcquisitions {
				acquisitionMap[acqNum] = ordinalAcquisitionNum
				ordinalAcquisitionNum++
			}
		}

		// Keep track of the instance number for each acquisition.
		instanceCounters := make(map[int]int)

		for _, file := range dicomFiles {
			ordinalAcq := acquisitionMap[file.AcquisitionNumber]
			instanceCounters[ordinalAcq]++ // Increment instance number for this acquisition.
			instanceNum := instanceCounters[ordinalAcq]

			// Format is AcquisitionOrdinal-InstanceOrdinalPadded.dcm.
			newName := fmt.Sprintf("%d-%04d.dcm", ordinalAcq, instanceNum)
			newPath := filepath.Join(seriesDir, newName)

			// Safely rename the file, handling case-insensitive filesystems.
			if err := safeRename(file.Path, newPath); err != nil {
				return fmt.Errorf("could not rename file %s to %s: %v", file.Path, newPath, err)
			}
		}
	}
	return nil
}

// safeRename handles renaming files, especially on case-insensitive filesystems.
func safeRename(source, destination string) error {
	// Check if the source and destination are the same file.
	srcInfo, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("could not stat source file: %w", err)
	}
	destInfo, err := os.Stat(destination)
	if err == nil {
		if os.SameFile(srcInfo, destInfo) {
			logger.Debugf("Skipping rename, source and destination are the same file: %s", source)
			return nil
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("could not stat destination file: %w", err)
	}

	// If we are here, it's safe to rename.
	return os.Rename(source, destination)
}
