package optimizer

import (
	"context"
	"fmt"
	"math"
	"sync"

	"github.com/c-bata/goptuna"
	goptunaCMAES "github.com/c-bata/goptuna/cmaes"
	goptunaSOBOL "github.com/c-bata/goptuna/sobol"
	goptunaTPE "github.com/c-bata/goptuna/tpe"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/cheggaaa/pb/v3"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// WARNING: the text here could only be lower cases
const (
	// HpOptimizerObjectiveEquity optimize the parameters to maximize equity gain
	HpOptimizerObjectiveEquity = "equity"
	// HpOptimizerObjectiveProfit optimize the parameters to maximize trading profit
	HpOptimizerObjectiveProfit = "profit"
	// HpOptimizerObjectiveVolume optimize the parameters to maximize trading volume
	HpOptimizerObjectiveVolume = "volume"
	// HpOptimizerObjectiveProfitFactor optimize the parameters to maximize profit factor
	HpOptimizerObjectiveProfitFactor = "profitfactor"
)

const (
	// HpOptimizerAlgorithmTPE is the implementation of Tree-structured Parzen Estimators
	HpOptimizerAlgorithmTPE = "tpe"
	// HpOptimizerAlgorithmCMAES is the implementation Covariance Matrix Adaptation Evolution Strategy
	HpOptimizerAlgorithmCMAES = "cmaes"
	// HpOptimizerAlgorithmSOBOL is the implementation Quasi-monte carlo sampling based on Sobol sequence
	HpOptimizerAlgorithmSOBOL = "sobol"
	// HpOptimizerAlgorithmRandom is the implementation random search
	HpOptimizerAlgorithmRandom = "random"
)

type HyperparameterOptimizeTrialResult struct {
	Value      fixedpoint.Value       `json:"value"`
	Parameters map[string]interface{} `json:"parameters"`
	ID         *int                   `json:"id,omitempty"`
	State      string                 `json:"state,omitempty"`
}

type HyperparameterOptimizeReport struct {
	Name       string                               `json:"studyName"`
	Objective  string                               `json:"objective"`
	Parameters map[string]string                    `json:"domains"`
	Best       *HyperparameterOptimizeTrialResult   `json:"best"`
	Trials     []*HyperparameterOptimizeTrialResult `json:"trials,omitempty"`
}

func buildBestHyperparameterOptimizeResult(study *goptuna.Study) *HyperparameterOptimizeTrialResult {
	val, _ := study.GetBestValue()
	params, _ := study.GetBestParams()
	return &HyperparameterOptimizeTrialResult{
		Value:      fixedpoint.NewFromFloat(val),
		Parameters: params,
	}
}

func buildHyperparameterOptimizeTrialResults(study *goptuna.Study) []*HyperparameterOptimizeTrialResult {
	trials, _ := study.GetTrials()
	results := make([]*HyperparameterOptimizeTrialResult, len(trials))
	for i, trial := range trials {
		trialId := trial.ID
		trialResult := &HyperparameterOptimizeTrialResult{
			ID:         &trialId,
			Value:      fixedpoint.NewFromFloat(trial.Value),
			Parameters: trial.Params,
		}
		results[i] = trialResult
	}
	return results
}

type HyperparameterOptimizer struct {
	SessionName string
	Config      *Config

	// Workaround for goptuna/tpe parameter suggestion. Remove this after fixed.
	// ref: https://github.com/c-bata/goptuna/issues/236
	paramSuggestionLock sync.Mutex
}

func (o *HyperparameterOptimizer) buildStudy(trialFinishChan chan goptuna.FrozenTrial) (*goptuna.Study, error) {
	var studyOpts = make([]goptuna.StudyOption, 0, 2)

	// maximum the profit, volume, equity gain, ...etc
	studyOpts = append(studyOpts, goptuna.StudyOptionDirection(goptuna.StudyDirectionMaximize))

	// disable search log and collect trial progress
	studyOpts = append(studyOpts, goptuna.StudyOptionLogger(nil))
	studyOpts = append(studyOpts, goptuna.StudyOptionTrialNotifyChannel(trialFinishChan))

	// the search algorithm
	var sampler goptuna.Sampler = nil
	var relativeSampler goptuna.RelativeSampler = nil
	switch o.Config.Algorithm {
	case HpOptimizerAlgorithmRandom:
		sampler = goptuna.NewRandomSampler()
	case HpOptimizerAlgorithmTPE:
		sampler = goptunaTPE.NewSampler()
	case HpOptimizerAlgorithmCMAES:
		relativeSampler = goptunaCMAES.NewSampler(goptunaCMAES.SamplerOptionNStartupTrials(5))
	case HpOptimizerAlgorithmSOBOL:
		relativeSampler = goptunaSOBOL.NewSampler()
	}
	if sampler != nil {
		studyOpts = append(studyOpts, goptuna.StudyOptionSampler(sampler))
	} else {
		studyOpts = append(studyOpts, goptuna.StudyOptionRelativeSampler(relativeSampler))
	}

	return goptuna.CreateStudy(o.SessionName, studyOpts...)
}

func (o *HyperparameterOptimizer) buildParamDomains() (map[string]string, []paramDomain) {
	labelPaths := make(map[string]string)
	domains := make([]paramDomain, 0, len(o.Config.Matrix))

	for _, selector := range o.Config.Matrix {
		var domain paramDomain
		switch selector.Type {
		case selectorTypeRange, selectorTypeRangeFloat:
			if selector.Step.IsZero() {
				domain = &floatRangeDomain{
					paramDomainBase: paramDomainBase{
						label: selector.Label,
						path:  selector.Path,
					},
					min: selector.Min.Float64(),
					max: selector.Max.Float64(),
				}
			} else {
				domain = &floatDiscreteRangeDomain{
					paramDomainBase: paramDomainBase{
						label: selector.Label,
						path:  selector.Path,
					},
					min:  selector.Min.Float64(),
					max:  selector.Max.Float64(),
					step: selector.Step.Float64(),
				}
			}
		case selectorTypeRangeInt:
			if selector.Step.IsZero() {
				domain = &intRangeDomain{
					paramDomainBase: paramDomainBase{
						label: selector.Label,
						path:  selector.Path,
					},
					min: selector.Min.Int(),
					max: selector.Max.Int(),
				}
			} else {
				domain = &intStepRangeDomain{
					paramDomainBase: paramDomainBase{
						label: selector.Label,
						path:  selector.Path,
					},
					min:  selector.Min.Int(),
					max:  selector.Max.Int(),
					step: selector.Step.Int(),
				}
			}
		case selectorTypeIterate, selectorTypeString:
			domain = &stringDomain{
				paramDomainBase: paramDomainBase{
					label: selector.Label,
					path:  selector.Path,
				},
				options: selector.Values,
			}
		case selectorTypeBool:
			domain = &boolDomain{
				paramDomainBase: paramDomainBase{
					label: selector.Label,
					path:  selector.Path,
				},
			}
		default:
			// unknown parameter type, skip
			continue
		}
		labelPaths[selector.Label] = selector.Path
		domains = append(domains, domain)
	}
	return labelPaths, domains
}

func (o *HyperparameterOptimizer) buildObjective(executor Executor, configJson []byte, paramDomains []paramDomain) goptuna.FuncObjective {
	var metricValueFunc MetricValueFunc
	switch o.Config.Objective {
	case HpOptimizerObjectiveProfit:
		metricValueFunc = TotalProfitMetricValueFunc
	case HpOptimizerObjectiveVolume:
		metricValueFunc = TotalVolume
	case HpOptimizerObjectiveEquity:
		metricValueFunc = TotalEquityDiff
	case HpOptimizerObjectiveProfitFactor:
		metricValueFunc = ProfitFactorMetricValueFunc
	}

	return func(trial goptuna.Trial) (float64, error) {
		trialConfig, err := func(trialConfig []byte) ([]byte, error) {
			o.paramSuggestionLock.Lock()
			defer o.paramSuggestionLock.Unlock()

			for _, domain := range paramDomains {
				if patch, err := domain.buildPatch(&trial); err != nil {
					return nil, err
				} else if patchedConfig, err := patch.ApplyIndent(trialConfig, "  "); err != nil {
					return nil, err
				} else {
					trialConfig = patchedConfig
				}
			}
			return trialConfig, nil
		}(configJson)
		if err != nil {
			return 0.0, err
		}

		summary, err := executor.Execute(trialConfig)
		if err != nil {
			return 0.0, err
		}
		// By config, the Goptuna optimize the parameters by maximize the objective output.
		return metricValueFunc(summary), nil
	}
}

func (o *HyperparameterOptimizer) Run(ctx context.Context, executor Executor, configJson []byte) (*HyperparameterOptimizeReport, error) {
	labelPaths, paramDomains := o.buildParamDomains()
	objective := o.buildObjective(executor, configJson, paramDomains)

	maxEvaluation := o.Config.MaxEvaluation
	numOfProcesses := o.Config.Executor.LocalExecutorConfig.MaxNumberOfProcesses
	if numOfProcesses > maxEvaluation {
		numOfProcesses = maxEvaluation
	}
	maxEvaluationPerProcess := maxEvaluation / numOfProcesses
	if maxEvaluation%numOfProcesses > 0 {
		maxEvaluationPerProcess++
	}

	trialFinishChan := make(chan goptuna.FrozenTrial, 128)
	allTrailFinishChan := make(chan struct{})
	bar := pb.Full.Start(maxEvaluation)
	bar.SetTemplateString(`{{ string . "log" | green}} | {{counters . }} {{bar . }} {{percent . }} {{etime . }} {{rtime . "ETA %s"}}`)

	go func() {
		defer close(allTrailFinishChan)
		var bestVal = math.Inf(-1)
		for result := range trialFinishChan {
			log.WithFields(logrus.Fields{"ID": result.ID, "evaluation": result.Value, "state": result.State}).Debug("trial finished")
			if result.State == goptuna.TrialStateFail {
				log.WithFields(result.Params).Errorf("failed at trial #%d", result.ID)
			}
			if result.Value > bestVal {
				bestVal = result.Value
			}
			bar.Set("log", fmt.Sprintf("best value: %v", bestVal))
			bar.Increment()
		}
	}()

	study, err := o.buildStudy(trialFinishChan)
	if err != nil {
		return nil, err
	}
	eg, studyCtx := errgroup.WithContext(ctx)
	study.WithContext(studyCtx)
	for i := 0; i < numOfProcesses; i++ {
		processEvaluations := maxEvaluationPerProcess
		if processEvaluations > maxEvaluation {
			processEvaluations = maxEvaluation
		}
		eg.Go(func() error {
			return study.Optimize(objective, processEvaluations)
		})
		maxEvaluation -= processEvaluations
	}
	if err := eg.Wait(); err != nil && ctx.Err() != context.Canceled {
		return nil, err
	}
	close(trialFinishChan)
	<-allTrailFinishChan
	bar.Finish()

	return &HyperparameterOptimizeReport{
		Name:       o.SessionName,
		Objective:  o.Config.Objective,
		Parameters: labelPaths,
		Best:       buildBestHyperparameterOptimizeResult(study),
		Trials:     buildHyperparameterOptimizeTrialResults(study),
	}, nil
}
