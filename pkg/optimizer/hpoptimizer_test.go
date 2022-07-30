package optimizer

import (
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"reflect"
	"testing"
)

func TestBuildParamDomains(t *testing.T) {
	var floatRangeDomainVerifier = func(domain paramDomain, expect SelectorConfig) bool {
		concrete := domain.(*floatRangeDomain)
		return *concrete == floatRangeDomain{
			paramDomainBase: paramDomainBase{label: expect.Label, path: expect.Path},
			min:             expect.Min.Float64(),
			max:             expect.Max.Float64(),
		}
	}
	var floatDiscreteRangeDomainVerifier = func(domain paramDomain, expect SelectorConfig) bool {
		concrete := domain.(*floatDiscreteRangeDomain)
		return *concrete == floatDiscreteRangeDomain{
			paramDomainBase: paramDomainBase{label: expect.Label, path: expect.Path},
			min:             expect.Min.Float64(),
			max:             expect.Max.Float64(),
			step:            expect.Step.Float64(),
		}
	}
	var intRangeDomainVerifier = func(domain paramDomain, expect SelectorConfig) bool {
		concrete := domain.(*intRangeDomain)
		return *concrete == intRangeDomain{
			paramDomainBase: paramDomainBase{label: expect.Label, path: expect.Path},
			min:             expect.Min.Int(),
			max:             expect.Max.Int(),
		}
	}
	var intStepRangeDomainVerifier = func(domain paramDomain, expect SelectorConfig) bool {
		concrete := domain.(*intStepRangeDomain)
		return *concrete == intStepRangeDomain{
			paramDomainBase: paramDomainBase{label: expect.Label, path: expect.Path},
			min:             expect.Min.Int(),
			max:             expect.Max.Int(),
			step:            expect.Step.Int(),
		}
	}
	var stringDomainVerifier = func(domain paramDomain, expect SelectorConfig) bool {
		concrete := domain.(*stringDomain)
		expectBase := paramDomainBase{label: expect.Label, path: expect.Path}
		if concrete.paramDomainBase != expectBase {
			return false
		}
		if len(concrete.options) != len(expect.Values) {
			return false
		}
		for i, item := range concrete.options {
			if item != expect.Values[i] {
				return false
			}
		}
		return true
	}
	var boolDomainVerifier = func(domain paramDomain, expect SelectorConfig) bool {
		concrete := domain.(*boolDomain)
		return *concrete == boolDomain{
			paramDomainBase: paramDomainBase{label: expect.Label, path: expect.Path},
		}
	}

	tests := []struct {
		config SelectorConfig
		verify func(domain paramDomain, expect SelectorConfig) bool
	}{
		{
			config: SelectorConfig{
				Type:   selectorTypeRange,
				Label:  "range label",
				Path:   "range path",
				Values: []string{"ignore", "ignore"},
				Min:    fixedpoint.NewFromFloat(7.0),
				Max:    fixedpoint.NewFromFloat(80.0),
				Step:   fixedpoint.NewFromFloat(0.0),
			},
			verify: floatRangeDomainVerifier,
		}, {
			config: SelectorConfig{
				Type:   selectorTypeRangeFloat,
				Label:  "rangeFloat label",
				Path:   "rangeFloat path",
				Values: []string{"ignore", "ignore"},
				Min:    fixedpoint.NewFromFloat(6.0),
				Max:    fixedpoint.NewFromFloat(10.0),
				Step:   fixedpoint.NewFromFloat(0.0),
			},
			verify: floatRangeDomainVerifier,
		}, {
			config: SelectorConfig{
				Type:   selectorTypeRangeFloat,
				Label:  "rangeDiscreteFloat label",
				Path:   "rangeDiscreteFloat path",
				Values: []string{"ignore", "ignore"},
				Min:    fixedpoint.NewFromFloat(6.0),
				Max:    fixedpoint.NewFromFloat(10.0),
				Step:   fixedpoint.NewFromFloat(2.0),
			},
			verify: floatDiscreteRangeDomainVerifier,
		}, {
			config: SelectorConfig{
				Type:   selectorTypeRangeInt,
				Label:  "rangeInt label",
				Path:   "rangeInt path",
				Values: []string{"ignore", "ignore"},
				Min:    fixedpoint.NewFromInt(3),
				Max:    fixedpoint.NewFromInt(100),
				Step:   fixedpoint.NewFromInt(0),
			},
			verify: intRangeDomainVerifier,
		}, {
			config: SelectorConfig{
				Type:   selectorTypeRangeInt,
				Label:  "rangeInt label",
				Path:   "rangeInt path",
				Values: []string{"ignore", "ignore"},
				Min:    fixedpoint.NewFromInt(3),
				Max:    fixedpoint.NewFromInt(100),
				Step:   fixedpoint.NewFromInt(7),
			},
			verify: intStepRangeDomainVerifier,
		}, {
			config: SelectorConfig{
				Type:   selectorTypeIterate,
				Label:  "iterate label",
				Path:   "iterate path",
				Values: nil,
				Min:    fixedpoint.NewFromInt(0),
				Max:    fixedpoint.NewFromInt(-8),
				Step:   fixedpoint.NewFromInt(-1),
			},
			verify: stringDomainVerifier,
		}, {
			config: SelectorConfig{
				Type:   selectorTypeString,
				Label:  "string label",
				Path:   "string path",
				Values: []string{"option1", "option2", "option3"},
				Min:    fixedpoint.NewFromInt(0),
				Max:    fixedpoint.NewFromInt(-8),
				Step:   fixedpoint.NewFromInt(-1),
			},
			verify: stringDomainVerifier,
		}, {
			config: SelectorConfig{
				Type:   selectorTypeBool,
				Label:  "bool label",
				Path:   "bool path",
				Values: []string{"ignore"},
				Min:    fixedpoint.NewFromInt(99),
				Max:    fixedpoint.NewFromInt(1064),
				Step:   fixedpoint.NewFromInt(-89),
			},
			verify: boolDomainVerifier,
		}, {
			config: SelectorConfig{
				Type:   "unknown type",
				Label:  "unknown label",
				Path:   "unknown path",
				Values: []string{"unknown option"},
				Min:    fixedpoint.NewFromInt(99),
				Max:    fixedpoint.NewFromFloat(1064),
				Step:   fixedpoint.NewFromInt(0),
			},
			verify: nil,
		},
	}

	selectors := make([]SelectorConfig, len(tests))
	expectLabelPaths := make(map[string]string)
	verifiers := make([]func(domain paramDomain) bool, 0, len(tests))
	for i, testItem := range tests {
		itemConfig, itemVerify := testItem.config, testItem.verify
		selectors[i] = itemConfig
		if itemVerify != nil {
			expectLabelPaths[testItem.config.Label] = testItem.config.Path
			verifiers = append(verifiers, func(domain paramDomain) bool {
				return itemVerify(domain, itemConfig)
			})
		}
	}
	optimizer := &HyperparameterOptimizer{Config: &Config{Matrix: selectors}}
	exactLabelPaths, exactParamDomains := optimizer.buildParamDomains()

	if !reflect.DeepEqual(exactLabelPaths, expectLabelPaths) {
		t.Errorf("expectLabelPaths=%v, exactLabelPaths=%v", expectLabelPaths, exactLabelPaths)
	}
	if len(exactParamDomains) != len(verifiers) {
		t.Errorf("expect %d param domains, got %d", len(verifiers), len(exactParamDomains))
	}
	for i, verifier := range verifiers {
		pd := exactParamDomains[i]
		if !verifier(pd) {
			t.Errorf("unexpect param domain at #%d: %#v", i, pd)
		}
	}
}
