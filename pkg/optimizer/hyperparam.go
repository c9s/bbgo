package optimizer

import (
	"fmt"
	"github.com/c-bata/goptuna"
	jsonpatch "github.com/evanphx/json-patch/v5"
)

type paramDomain interface {
	buildPatch(trail *goptuna.Trial) (jsonpatch.Patch, error)
}

type paramDomainBase struct {
	label string
	path  string
}

type intRangeDomain struct {
	paramDomainBase
	min int
	max int
}

func (d *intRangeDomain) buildPatch(trial *goptuna.Trial) (jsonpatch.Patch, error) {
	val, err := trial.SuggestInt(d.label, d.min, d.max)
	if err != nil {
		return nil, err
	}
	jsonOp := []byte(reformatJson(fmt.Sprintf(`[{"op": "replace", "path": "%s", "value": %v }]`, d.path, val)))
	return jsonpatch.DecodePatch(jsonOp)
}

type intStepRangeDomain struct {
	paramDomainBase
	min  int
	max  int
	step int
}

func (d *intStepRangeDomain) buildPatch(trial *goptuna.Trial) (jsonpatch.Patch, error) {
	val, err := trial.SuggestStepInt(d.label, d.min, d.max, d.step)
	if err != nil {
		return nil, err
	}
	jsonOp := []byte(reformatJson(fmt.Sprintf(`[{"op": "replace", "path": "%s", "value": %v }]`, d.path, val)))
	return jsonpatch.DecodePatch(jsonOp)
}

type floatRangeDomain struct {
	paramDomainBase
	min float64
	max float64
}

func (d *floatRangeDomain) buildPatch(trial *goptuna.Trial) (jsonpatch.Patch, error) {
	val, err := trial.SuggestFloat(d.label, d.min, d.max)
	if err != nil {
		return nil, err
	}
	jsonOp := []byte(reformatJson(fmt.Sprintf(`[{"op": "replace", "path": "%s", "value": %v }]`, d.path, val)))
	return jsonpatch.DecodePatch(jsonOp)
}

type floatDiscreteRangeDomain struct {
	paramDomainBase
	min  float64
	max  float64
	step float64
}

func (d *floatDiscreteRangeDomain) buildPatch(trial *goptuna.Trial) (jsonpatch.Patch, error) {
	val, err := trial.SuggestDiscreteFloat(d.label, d.min, d.max, d.step)
	if err != nil {
		return nil, err
	}
	jsonOp := []byte(reformatJson(fmt.Sprintf(`[{"op": "replace", "path": "%s", "value": %v }]`, d.path, val)))
	return jsonpatch.DecodePatch(jsonOp)
}

type stringDomain struct {
	paramDomainBase
	options []string
}

func (d *stringDomain) buildPatch(trial *goptuna.Trial) (jsonpatch.Patch, error) {
	val, err := trial.SuggestCategorical(d.label, d.options)
	if err != nil {
		return nil, err
	}
	jsonOp := []byte(reformatJson(fmt.Sprintf(`[{"op": "replace", "path": "%s", "value": "%v" }]`, d.path, val)))
	return jsonpatch.DecodePatch(jsonOp)
}

type boolDomain struct {
	paramDomainBase
}

func (d *boolDomain) buildPatch(trial *goptuna.Trial) (jsonpatch.Patch, error) {
	valStr, err := trial.SuggestCategorical(d.label, []string{"false", "true"})
	if err != nil {
		return nil, err
	}
	jsonOp := []byte(reformatJson(fmt.Sprintf(`[{"op": "replace", "path": "%s", "value": %s }]`, d.path, valStr)))
	return jsonpatch.DecodePatch(jsonOp)
}
