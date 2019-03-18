package service_test

import (
	"encoding/json"
	"fmt"
	"github.com/cloudfoundry-incubator/cf-test-helpers/workflowhelpers"
	"github.com/davecgh/go-spew/spew"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cloudfoundry-incubator/cf-test-helpers/config"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/pivotal-cf/cf-redis-smoke-tests/retry"
	"github.com/pivotal-cf/cf-redis-smoke-tests/service/reporter"
)

type retryConfig struct {
	BaselineMilliseconds uint   `json:"baseline_interval_milliseconds"`
	Attempts             uint   `json:"max_attempts"`
	BackoffAlgorithm     string `json:"backoff"`
}

func (rc retryConfig) Backoff() retry.Backoff {
	baseline := time.Duration(rc.BaselineMilliseconds) * time.Millisecond

	algorithm := strings.ToLower(rc.BackoffAlgorithm)

	switch algorithm {
	case "linear":
		return retry.Linear(baseline)
	case "exponential":
		return retry.Linear(baseline)
	default:
		return retry.None(baseline)
	}
}

func (rc retryConfig) MaxRetries() int {
	return int(rc.Attempts)
}

type redisTestConfig struct {
	config.Config

	ServiceName string      `json:"service_name"`
	PlanNames   []string    `json:"plan_names"`
	Retry       retryConfig `json:"retry"`
}

func loadRedisTestConfig(path string) redisTestConfig {
	file, err := os.Open(path)
	Expect(err).NotTo(HaveOccurred())

	defer file.Close()

	testConfig := redisTestConfig{}
	err = json.NewDecoder(file).Decode(&testConfig)
	Expect(err).NotTo(HaveOccurred())

	testConfig.Config.TimeoutScale = 3

	return testConfig
}

var (
	configPath   = os.Getenv("CONFIG_PATH")

	redisConfig  redisTestConfig

	smokeTestReporter *reporter.SmokeTestReport

	wfh *workflowhelpers.ReproducibleTestSuiteSetup
)

func TestService(t *testing.T) {
	smokeTestReporter = new(reporter.SmokeTestReport)

	testReporter := []Reporter{
		Reporter(smokeTestReporter),
	}

	BeforeSuite(func() {
		redisConfig  = loadRedisTestConfig(configPath)

		fmt.Println("------------------------------------------")
		fmt.Println("redisConfig:")
		spew.Dump(redisConfig)
		fmt.Println("------------------------------------------")

		wfh = workflowhelpers.NewTestSuiteSetup(&redisConfig.Config)

		beforeSuiteSteps := []*reporter.Step{
			reporter.NewStep(
				"Setup test suite",
				wfh.Setup,
			),
		}

		smokeTestReporter.RegisterBeforeSuiteSteps(beforeSuiteSteps)
		for _, task := range beforeSuiteSteps {
			task.Perform()
		}
	})

	AfterSuite(func() {
		afterSuiteSteps := []*reporter.Step{
			reporter.NewStep(
				"Tear down test suite",
				wfh.Teardown,
			),
		}

		smokeTestReporter.RegisterAfterSuiteSteps(afterSuiteSteps)
		for _, task := range afterSuiteSteps {
			task.Perform()
		}
	})

	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "P-Redis Smoke Tests", testReporter)
}
