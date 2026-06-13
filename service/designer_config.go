package service

import (
	"encoding/json"
	"os"
	"strings"
)

type DesignerConfig struct {
	FieldMapping map[string]string `json:"fieldMapping"`
	Records      []DesignerRecord  `json:"records"`
}

type DesignerRecord struct {
	Name              string `json:"name"`
	DesignType        string `json:"designType"`
	SettlementCompany string `json:"settlementCompany"`
}

func LoadDesignerConfig(path string) (*DesignerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg DesignerConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func BuildDesignerRecordMap(records []DesignerRecord) map[string]DesignerRecord {
	recordMap := make(map[string]DesignerRecord, len(records))
	for _, record := range records {
		name := strings.TrimSpace(record.Name)
		if name == "" {
			continue
		}
		recordMap[name] = record
	}
	return recordMap
}

func FindDesignerRecord(designerName string, recordMap map[string]DesignerRecord) (DesignerRecord, bool) {
	name := strings.TrimSpace(designerName)
	if name == "" {
		return DesignerRecord{}, false
	}

	record, ok := recordMap[name]
	return record, ok
}
