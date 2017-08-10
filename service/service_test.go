package service_test

import (
	"fmt"
	"strings"
	"time"

	"github.com/pborman/uuid"
	"github.com/pivotal-cf/cf-redis-smoke-tests/redis"
	"github.com/pivotal-cf/cf-redis-smoke-tests/service/reporter"

	"github.com/pivotal-cf-experimental/cf-test-helpers/services"
	smokeTestCF "github.com/pivotal-cf/cf-redis-smoke-tests/cf"

	"encoding/json"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type CFTestContext struct {
	Org, Space string
}

var _ = Describe("Redis Service", func() {
	var (
		testCF = smokeTestCF.CF{
			ShortTimeout: time.Minute * 3,
			LongTimeout:  time.Minute * 15,
			RetryBackoff: redisConfig.Retry.Backoff(),
			MaxRetries:   redisConfig.Retry.MaxRetries(),
		}

		retryInterval = time.Second

		appPath             = "../assets/cf-redis-example-app"
		serviceInstanceName string
		appName             string
		planName            string
		securityGroupName   string

		cfTestContext CFTestContext

		context services.Context
	)

	SynchronizedBeforeSuite(func() []byte {
		context = services.NewContext(cfTestConfig, "redis-test")

		createQuotaArgs := []string{
			"-m", "10G",
			"-r", "1000",
			"-s", "100",
			"--allow-paid-service-plans",
		}

		regularContext := context.RegularUserContext()

		beforeSuiteSteps := []*reporter.Step{
			reporter.NewStep(
				"Connect to CloudFoundry",
				testCF.API(cfTestConfig.ApiEndpoint, cfTestConfig.SkipSSLValidation),
			),
			reporter.NewStep(
				"Log in as admin",
				testCF.Auth(cfTestConfig.AdminUser, cfTestConfig.AdminPassword),
			),
			reporter.NewStep(
				"Create 'redis-smoke-tests' quota",
				testCF.CreateQuota("redis-smoke-test-quota", createQuotaArgs...),
			),
			reporter.NewStep(
				fmt.Sprintf("Create '%s' org", regularContext.Org),
				testCF.CreateOrg(regularContext.Org, "redis-smoke-test-quota"),
			),
			reporter.NewStep(
				fmt.Sprintf("Enable service access for '%s' org", regularContext.Org),
				testCF.EnableServiceAccess(regularContext.Org, redisConfig.ServiceName),
			),
			reporter.NewStep(
				fmt.Sprintf("Target '%s' org", regularContext.Org),
				testCF.TargetOrg(regularContext.Org),
			),
			reporter.NewStep(
				fmt.Sprintf("Create '%s' space", regularContext.Space),
				testCF.CreateSpace(regularContext.Space),
			),
			reporter.NewStep(
				fmt.Sprintf("Target '%s' org and '%s' space", regularContext.Org, regularContext.Space),
				testCF.TargetOrgAndSpace(regularContext.Org, regularContext.Space),
			),
		}

		smokeTestReporter.RegisterBeforeSuiteSteps(beforeSuiteSteps)

		for _, task := range beforeSuiteSteps {
			task.Perform()
		}

		cfTestContext = CFTestContext{
			Org:   regularContext.Org,
			Space: regularContext.Space,
		}

		rawTestContext, err := json.Marshal(cfTestContext)
		Expect(err).NotTo(HaveOccurred())

		return rawTestContext
	}, func(data []byte) {
		err := json.Unmarshal(data, &cfTestContext)
		Expect(err).NotTo(HaveOccurred())
	})

	BeforeEach(func() {
		appName = randomName()
		serviceInstanceName = randomName()
		securityGroupName = randomName()

		pushArgs := []string{
			"-m", "256M",
			"-p", appPath,
			"-s", "cflinuxfs2",
			"-no-start",
		}

		specSteps := []*reporter.Step{
			reporter.NewStep(
				"Log in as admin",
				testCF.Auth(cfTestConfig.AdminUser, cfTestConfig.AdminPassword),
			),
			reporter.NewStep(
				fmt.Sprintf("Target '%s' org and '%s' space", cfTestContext.Org, cfTestContext.Space),
				testCF.TargetOrgAndSpace(cfTestContext.Org, cfTestContext.Space),
			),
			reporter.NewStep(
				"Push the redis sample app to Cloud Foundry",
				testCF.Push(appName, pushArgs...),
			),
		}

		smokeTestReporter.ClearSpecSteps()
		smokeTestReporter.RegisterSpecSteps(specSteps)

		for _, task := range specSteps {
			task.Perform()
		}
	})

	AfterEach(func() {
		specSteps := []*reporter.Step{
			reporter.NewStep(
				fmt.Sprintf("Unbind the %q plan instance", planName),
				testCF.UnbindService(appName, serviceInstanceName),
			),
			reporter.NewStep(
				fmt.Sprintf("Delete the %q plan instance", planName),
				testCF.DeleteService(serviceInstanceName),
			),
			reporter.NewStep(
				fmt.Sprintf("Ensure service instance for plan %q has been deleted", planName),
				testCF.EnsureServiceInstanceGone(serviceInstanceName),
			),
			reporter.NewStep(
				"Delete the app",
				testCF.Delete(appName),
			),
			reporter.NewStep(
				fmt.Sprintf("Delete security group '%s'", securityGroupName),
				testCF.DeleteSecurityGroup(securityGroupName),
			),
		}

		smokeTestReporter.RegisterSpecSteps(specSteps)

		for _, task := range specSteps {
			task.Perform()
		}
	})

	SynchronizedAfterSuite(func() {}, func() {
		afterSuiteSteps := []*reporter.Step{
			reporter.NewStep(
				"Ensure no service-instances left",
				testCF.EnsureAllServiceInstancesGone(),
			),
			reporter.NewStep(
				fmt.Sprintf("Delete org '%s'", cfTestContext.Org),
				testCF.DeleteOrg(cfTestContext.Org),
			),
			reporter.NewStep(
				"Log out",
				testCF.Logout(),
			),
		}

		smokeTestReporter.RegisterAfterSuiteSteps(afterSuiteSteps)

		for _, task := range afterSuiteSteps {
			task.Perform()
		}
	})

	AssertLifeCycleBehavior := func(planName string) {
		It(strings.ToUpper(planName)+": create, bind to, write to, read from, unbind, and destroy a service instance", func() {
			var skip bool

			uri := fmt.Sprintf("https://%s.%s", appName, cfTestConfig.AppsDomain)
			app := redis.NewApp(uri, testCF.ShortTimeout, retryInterval)

			serviceCreateStep := reporter.NewStep(
				fmt.Sprintf("Create a '%s' plan instance of Redis\n    Please refer to http://docs.pivotal.io/redis/smoke-tests.html for more help on diagnosing this issue", planName),
				testCF.CreateService(redisConfig.ServiceName, planName, serviceInstanceName, &skip),
			)

			smokeTestReporter.RegisterSpecSteps([]*reporter.Step{serviceCreateStep})

			specSteps := []*reporter.Step{
				reporter.NewStep(
					fmt.Sprintf("Bind the redis sample app '%s' to the '%s' plan instance '%s' of Redis", appName, planName, serviceInstanceName),
					testCF.BindService(appName, serviceInstanceName),
				),
				reporter.NewStep(
					fmt.Sprintf("Create and bind security group '%s' for running smoke tests", securityGroupName),
					testCF.CreateAndBindSecurityGroup(securityGroupName, appName, cfTestContext.Org, cfTestContext.Space),
				),
				reporter.NewStep(
					"Start the app",
					testCF.Start(appName),
				),
				reporter.NewStep(
					"Verify that the app is responding",
					app.IsRunning(),
				),
				reporter.NewStep(
					"Write a key/value pair to Redis",
					app.Write("mykey", "myvalue"),
				),
				reporter.NewStep(
					"Read the key/value pair back",
					app.ReadAssert("mykey", "myvalue"),
				),
			}

			smokeTestReporter.RegisterSpecSteps(specSteps)

			serviceCreateStep.Perform()
			serviceCreateStep.Description = fmt.Sprintf("Create a '%s' plan instance of Redis", planName)

			if skip {
				serviceCreateStep.Result = "SKIPPED"
			} else {
				for _, task := range specSteps {
					task.Perform()
				}
			}
		})
	}

	Context("for each plan", func() {
		for _, planName = range redisConfig.PlanNames {
			AssertLifeCycleBehavior(planName)
		}
	})
})

func randomName() string {
	return uuid.NewRandom().String()
}
