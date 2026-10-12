package translate

import (
	"errors"
	"strings"
)

func ValidateBatch(source []Segment, translated map[string]string) error {
	if len(source) != len(translated) {
		return errors.New("译文块数量不一致")
	}
	for _, segment := range source {
		value, ok := translated[segment.ID]
		if !ok || strings.TrimSpace(value) == "" {
			return errors.New("译文缺少块")
		}
		if strings.TrimSpace(value) == strings.TrimSpace(segment.Text) {
			return errors.New("译文与原文相同")
		}
		if err := ValidateInline(segment.Text, value); err != nil {
			return err
		}
	}
	return nil
}
func Split(values []Segment) ([]Segment, []Segment) {
	middle := (len(values) + 1) / 2
	return append([]Segment{}, values[:middle]...), append([]Segment{}, values[middle:]...)
}
