package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

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
		// InstanceNumber can also be optional, default to 0
		instanceNumberStr = "0"
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

func getElementValue(dataset dicom.Dataset, tag tag.Tag) (string, error) {
	element, err := dataset.FindElementByTag(tag)
	if err != nil {
		return "", fmt.Errorf("could not find tag %v", tag)
	}
	return element.Value.String(), nil
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

		// Map original acquisition numbers to new ordinal ones starting from 1
		acquisitionMap := make(map[int]int)
		ordinalAcquisitionNum := 1
		if len(dicomFiles) > 0 {
			// Get unique, sorted acquisition numbers
			uniqueAcquisitions := []int{}
			lastAcq := dicomFiles[0].AcquisitionNumber
			uniqueAcquisitions = append(uniqueAcquisitions, lastAcq)
			for _, file := range dicomFiles {
				if file.AcquisitionNumber != lastAcq {
					lastAcq = file.AcquisitionNumber
					uniqueAcquisitions = append(uniqueAcquisitions, lastAcq)
				}
			}
			// Create the map
			for _, acqNum := range uniqueAcquisitions {
				acquisitionMap[acqNum] = ordinalAcquisitionNum
				ordinalAcquisitionNum++
			}
		}

		instanceCounter := 0
		lastAcquisitionNumber := -1 // Initialize with a value that won't match any real acquisition number

		for _, file := range dicomFiles {
			if file.AcquisitionNumber != lastAcquisitionNumber {
				instanceCounter = 1 // Reset for new acquisition
				lastAcquisitionNumber = file.AcquisitionNumber
			} else {
				instanceCounter++
			}

			ordinalAcq := acquisitionMap[file.AcquisitionNumber]
			// Format is Acquisition-Instance, with instance number padded.
			newName := fmt.Sprintf("%04d-%04d.dcm", ordinalAcq, instanceCounter)
			newPath := filepath.Join(seriesDir, newName)

			// Check if source and destination are the same file to handle case-insensitive filesystems.
			srcInfo, err := os.Stat(file.Path)
			if err != nil {
				return fmt.Errorf("could not stat source file %s: %v", file.Path, err)
			}
			destInfo, err := os.Stat(newPath)
			if err == nil {
				// Destination exists.
				if os.SameFile(srcInfo, destInfo) {
					logger.Debugf("Skipping rename, source and destination are the same: %s", file.Path)
					continue
				}
			} else if !os.IsNotExist(err) {
				// An unexpected error occurred when stating the destination.
				return fmt.Errorf("could not stat destination path %s: %v", newPath, err)
			}

			if err := os.Rename(file.Path, newPath); err != nil {
				// On Windows, rename can fail if the destination exists. Try move/copy+delete as a fallback.
				// For this simple case, we just log the error.
				return fmt.Errorf("could not rename file %s to %s: %v", file.Path, newPath, err)
			}
		}
	}
	return nil
}
