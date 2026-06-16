package gpt

import "testing"

func TestParseECGInterpretation_Valid(t *testing.T) {
	raw := `{"interpretation_md":"## Итог\n- Синусовый ритм.","rhythm":{"code":"SINUS_GROUP","label_ru":"синусовый ритм","agrees_with_classifier":true}}`
	res, err := ParseECGInterpretation(raw)
	if err != nil {
		t.Fatal(err)
	}
	if res.InterpretationMD != "## Итог\n- Синусовый ритм." {
		t.Errorf("interpretation_md wrong: %q", res.InterpretationMD)
	}
	if res.Rhythm == nil || res.Rhythm.Code != "SINUS_GROUP" || !res.Rhythm.AgreesWithClassifier {
		t.Errorf("rhythm parsed wrong: %+v", res.Rhythm)
	}
}

func TestParseECGInterpretation_StripsCodeFences(t *testing.T) {
	raw := "```json\n{\"interpretation_md\":\"## Итог\\n- Текст.\",\"rhythm\":{\"code\":\"VTAC\",\"label_ru\":\"ЖТ\",\"agrees_with_classifier\":false}}\n```"
	res, err := ParseECGInterpretation(raw)
	if err != nil {
		t.Fatal(err)
	}
	if res.Rhythm == nil || res.Rhythm.AgreesWithClassifier {
		t.Errorf("expected disagreeing VTAC rhythm, got %+v", res.Rhythm)
	}
}

func TestParseECGInterpretation_MissingMarkdownIsError(t *testing.T) {
	if _, err := ParseECGInterpretation(`{"rhythm":{"code":"AFIB"}}`); err == nil {
		t.Error("expected error when interpretation_md is missing")
	}
	if _, err := ParseECGInterpretation(`## Итог plain markdown, not JSON`); err == nil {
		t.Error("expected error for non-JSON payload")
	}
}
