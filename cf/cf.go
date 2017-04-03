package cf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
	helpersCF "github.com/pivotal-cf-experimental/cf-test-helpers/cf"
	"github.com/pivotal-cf/on-demand-service-broker/system_tests/cf_helpers"
)

//CF is a testing wrapper around the cf cli
type CF struct {
	ShortTimeout time.Duration
	LongTimeout  time.Duration
}

//API is equivalent to `cf api {endpoint} [--skip-ssl-validation]`
func (cf *CF) API(endpoint string, skipSSLValidation bool) func() {
	return func() {
		apiCmd := []string{"api", endpoint}

		if skipSSLValidation {
			apiCmd = append(apiCmd, "--skip-ssl-validation")
		}

		Eventually(helpersCF.Cf(apiCmd...), cf.ShortTimeout).Should(
			gexec.Exit(0),
			`{"FailReason": "Failed to target Cloud Foundry"}`,
		)
	}
}

//Auth is equivalent to `cf auth {user} {password}`
func (cf *CF) Auth(user, password string) func() {
	return func() {
		Eventually(helpersCF.Cf("auth", user, password), cf.ShortTimeout).Should(
			gexec.Exit(0),
			"{\"FailReason\": \"Failed to `cf auth` with target Cloud Foundry\"}",
		)
	}
}

//CreateQuota is equivalent to `cf create-quota {name} [args...]`
func (cf *CF) CreateQuota(name string, args ...string) func() {
	return func() {
		cfArgs := []string{"create-quota", name}
		cfArgs = append(cfArgs, args...)
		Eventually(helpersCF.Cf(cfArgs...), cf.ShortTimeout).Should(
			gexec.Exit(0),
			"{\"FailReason\": \"Failed to `cf create-quota` with target Cloud Foundry\"}",
		)
	}
}

//CreateOrg is equivalent to `cf create-org {org} -q {quota}`
func (cf *CF) CreateOrg(org, quota string) func() {
	return func() {
		Eventually(helpersCF.Cf("create-org", org, "-q", quota), cf.ShortTimeout).Should(
			gexec.Exit(0),
			`{"FailReason": "Failed to create CF test org"}`,
		)
	}
}

//EnableServiceAccess is equivalent to `cf enable-service-access -o {org} {service-offering}`
//In order to run enable-service-access idempotently we disable-service-access before.
func (cf *CF) EnableServiceAccess(org, service string) func() {
	return func() {
		Eventually(helpersCF.Cf("disable-service-access", "-o", org, service), cf.ShortTimeout).Should(
			gexec.Exit(0),
			`{"FailReason": "Failed to disable service access for CF test org"}`,
		)
		Eventually(helpersCF.Cf("enable-service-access", "-o", org, service), cf.ShortTimeout).Should(
			gexec.Exit(0),
			`{"FailReason": "Failed to enable service access for CF test org"}`,
		)
	}
}

//TargetOrg is equivalent to `cf target -o {org}`
func (cf *CF) TargetOrg(org string) func() {
	return func() {
		Eventually(helpersCF.Cf("target", "-o", org), cf.ShortTimeout).Should(
			gexec.Exit(0),
			`{"FailReason": "Failed to target test org"}`,
		)
	}
}

//TargetOrgAndSpace is equivalent to `cf target -o {org} -s {space}`
func (cf *CF) TargetOrgAndSpace(org, space string) func() {
	return func() {
		Eventually(helpersCF.Cf("target", "-o", org, "-s", space), cf.ShortTimeout).Should(
			gexec.Exit(0),
			`{"FailReason": "Failed to target test org"}`,
		)
	}
}

//CreateSpace is equivalent to `cf create-space {space}`
func (cf *CF) CreateSpace(space string) func() {
	return func() {
		Eventually(helpersCF.Cf("create-space", space), cf.ShortTimeout).Should(
			gexec.Exit(0),
			`{"FailReason": "Failed to create CF test space"}`,
		)
	}
}

//CreateSecurityGroup is equivalent to `cf create-security-group {securityGroup} {configPath}`
func (cf *CF) CreateAndBindSecurityGroup(securityGroup, appName, org, space string) func() {
	return func() {
		appGuid := cf.getAppGuid(appName)

		host, port := cf.getBindingCredentials(appGuid)

		sgFile, err := ioutil.TempFile("", "smoke-test-security-group-")
		Expect(err).NotTo(HaveOccurred())
		defer sgFile.Close()
		defer os.Remove(sgFile.Name())

		sgs := []struct {
			Protocol    string `json:"protocol"`
			Destination string `json:"destination"`
			Ports       string `json:"ports"`
		}{
			{"tcp", host, fmt.Sprintf("%d", port)},
		}

		err = json.NewEncoder(sgFile).Encode(sgs)
		Expect(err).NotTo(HaveOccurred(), `{"FailReason": "Failed to encode security groups"}`)

		Eventually(helpersCF.Cf("create-security-group", securityGroup, sgFile.Name()), cf.ShortTimeout).Should(
			gexec.Exit(0),
			`{"FailReason": "Failed to create security group"}`,
		)

		Eventually(helpersCF.Cf("bind-security-group", securityGroup, org, space), cf.ShortTimeout).Should(
			gexec.Exit(0),
			`{"FailReason": "Failed to bind security group to space"}`,
		)
	}
}

//DeleteSecurityGroup is equivalent to `cf delete-security-group {securityGroup} -f`
func (cf *CF) DeleteSecurityGroup(securityGroup string) func() {
	return func() {
		Eventually(helpersCF.Cf("delete-security-group", securityGroup, "-f"), cf.ShortTimeout).Should(
			gexec.Exit(0),
			`{"FailReason": "Failed to delete security group"}`,
		)
	}
}

//CreateUser is equivalent to `cf create-user {name} {password}`
func (cf *CF) CreateUser(name, password string) func() {
	return func() {
		createUserCmd := helpersCF.Cf("create-user", name, password)
		Eventually(createUserCmd, cf.ShortTimeout).Should(gexec.Exit())
		if createUserCmd.ExitCode() != 0 {
			Expect(createUserCmd.Out).To(
				gbytes.Say("scim_resource_already_exists"),
				`{"FailReason": "Failed to create user"}`,
			)
		}
	}
}

//DeleteUser is equivalent to `cf delete-user -f {name}`
func (cf *CF) DeleteUser(name string) func() {
	return func() {
		Eventually(helpersCF.Cf("delete-user", "-f", name), cf.ShortTimeout).Should(
			gexec.Exit(0),
			`{"FailReason": "Failed to delete user"}`,
		)
	}
}

//SetSpaceRole is equivalent to `cf set-space-role {name} {org} {space} {role}`
func (cf *CF) SetSpaceRole(name, org, space, role string) func() {
	return func() {
		Eventually(helpersCF.Cf("set-space-role", name, org, space, role), cf.ShortTimeout).Should(
			gexec.Exit(0),
			`{"FailReason": "Failed to set space role"}`,
		)
	}
}

//Push is equivalent to `cf push {appName} [args...]`
func (cf *CF) Push(appName string, args ...string) func() {
	pushArgs := []string{"push", appName}
	pushArgs = append(pushArgs, args...)
	return func() {
		Eventually(helpersCF.Cf(pushArgs...), cf.ShortTimeout).Should(
			gexec.Exit(0),
			"{\"FailReason\": \"Failed to `cf push` test app\"}",
		)
	}
}

//Delete is equivalent to `cf delete {appName} -f`
func (cf *CF) Delete(appName string) func() {
	return func() {
		Eventually(helpersCF.Cf("delete", appName, "-f", "-r"), cf.ShortTimeout).Should(
			gexec.Exit(0),
			"{\"FailReason\": \"Failed to `cf delete` test app\"}",
		)
	}
}

//CreateService is equivalent to `cf create-service {serviceName} {planName} {instanceName}`
func (cf *CF) CreateService(serviceName, planName, instanceName string, skip *bool) func() {
	return func() {
		session := helpersCF.Cf("create-service", serviceName, planName, instanceName)
		session.Wait(cf.ShortTimeout)
		createServiceStdout := session.Out

		defer createServiceStdout.CancelDetects()
		select {
		case <-createServiceStdout.Detect("FAILED"):
			Eventually(session, cf.ShortTimeout).Should(
				gbytes.Say("instance limit for this service has been reached"),
				`{"FailReason": "Failed to bind Redis service instance to test app"}`,
			)
			Eventually(session, cf.ShortTimeout).Should(gexec.Exit(1))
			fmt.Printf("No Plan Instances available for testing %s plan\n", planName)
			*skip = true
		case <-createServiceStdout.Detect("OK"):
			Eventually(session, cf.ShortTimeout).Should(
				gexec.Exit(0),
				`{"FailReason": "Failed to create Redis service instance"}`,
			)
			cf_helpers.AwaitServiceCreation(instanceName)
		}
	}
}

//DeleteService is equivalent to `cf delete-service {instanceName} -f`
func (cf *CF) DeleteService(instanceName string) func() {
	return func() {
		Eventually(helpersCF.Cf("delete-service", "-f", instanceName), cf.ShortTimeout).Should(
			gexec.Exit(0),
			fmt.Sprintf(`{"FailReason": "Failed to delete service %s"}`, instanceName),
		)
	}
}

//EnsureDeleteService is equivalent to `cf delete-service {instanceName} -f`
func (cf *CF) EnsureDeleteService(instanceName string) func() {
	return func() {
		// fmt.Println("STOP NGINX NOW") // DO NOT COMMIT ME
		// time.Sleep(20 * time.Second)  // DO NOT COMMIT ME
		for retry := 0; retry < 20; retry++ {
			session := helpersCF.Cf("delete-service", "-f", instanceName)
			Eventually(session).Should(gexec.Exit())
			if session.ExitCode() == 0 {
				Eventually(helpersCF.Cf("service", instanceName), cf.LongTimeout).Should(gbytes.Say(fmt.Sprintf("Service instance %s not found", instanceName)))
				fmt.Println("Successfully deleted service instance ", instanceName) // DO NOT COMMIT ME
				return
			}
			fmt.Printf("Failed to delete service instance %s this time, will try again\n", instanceName) // DO NOT COMMIT ME
			time.Sleep(1 * time.Second)
		}
		panic(fmt.Sprintf("Failed to delete service instance %s.", instanceName))
	}
}

func (cf *CF) EnsureNoServices() func() {
	return func() {
		Eventually(helpersCF.Cf("services"), cf.LongTimeout).Should(gbytes.Say("No services found"))
	}
}

//BindService is equivalent to `cf bind-service {appName} {instanceName}`
func (cf *CF) BindService(appName, instanceName string) func() {
	return func() {
		Eventually(helpersCF.Cf("bind-service", appName, instanceName), cf.ShortTimeout).Should(
			gexec.Exit(0),
			`{"FailReason": "Failed to bind Redis service instance to test app"}`,
		)
	}
}

//UnbindService is equivalent to `cf unbind-service {appName} {instanceName}`
func (cf *CF) UnbindService(appName, instanceName string) func() {
	return func() {
		Eventually(helpersCF.Cf("unbind-service", appName, instanceName), cf.ShortTimeout).Should(
			gexec.Exit(0),
			fmt.Sprintf(`{"FailReason": "Failed to unbind %s instance from %s"}`, instanceName, appName),
		)
	}
}

//Start is equivalent to `cf start {appName}`
func (cf *CF) Start(appName string) func() {
	return func() {
		Eventually(helpersCF.Cf("start", appName), cf.LongTimeout).Should(
			gexec.Exit(0),
			`{"FailReason": "Failed to start test app"}`,
		)
	}
}

//SetEnv is equivalent to `cf set-env {appName} {envVarName} {instanceName}`
func (cf *CF) SetEnv(appName, environmentVariable, instanceName string) func() {
	return func() {
		Eventually(helpersCF.Cf("set-env", appName, environmentVariable, instanceName), cf.ShortTimeout).Should(
			gexec.Exit(0),
			`{"FailReason": "Failed to set environment variable for test app"}`,
		)
	}
}

//Logout is equivalent to `cf logout`
func (cf *CF) Logout() func() {
	return func() {
		Eventually(helpersCF.Cf("logout")).Should(
			gexec.Exit(0),
			`{"FailReason": "Failed to logout"}`,
		)
	}
}

func (cf *CF) getAppGuid(appName string) string {
	session := helpersCF.Cf("app", "--guid", appName)
	Eventually(session, cf.ShortTimeout).Should(gexec.Exit(0), `{"FailReason": "Failed to retrieve GUID for app"}`)

	return strings.Trim(string(session.Out.Contents()), " \n")
}

func (cf *CF) getBindingCredentials(appGuid string) (string, int) {
	session := helpersCF.Cf("curl", fmt.Sprintf("/v2/apps/%s/service_bindings", appGuid))
	Eventually(session, cf.ShortTimeout).Should(gexec.Exit(0), `{"FailReason": "Failed to retrieve service bindings for app"}`)

	var resp = new(struct {
		Resources []struct {
			Entity struct {
				Credentials struct {
					Host string
					Port int
				}
			}
		}
	})

	err := json.NewDecoder(bytes.NewBuffer(session.Out.Contents())).Decode(resp)
	Expect(err).NotTo(HaveOccurred(), `{"FailReason": "Failed to decode service binding response"}`)
	Expect(resp.Resources).To(HaveLen(1), `{"FailReason": "Invalid binding response, expected exactly one binding"}`)

	host, port := resp.Resources[0].Entity.Credentials.Host, resp.Resources[0].Entity.Credentials.Port
	Expect(host).NotTo(BeEmpty(), `{"FailReason": "Invalid binding, missing host"}`)
	Expect(port).NotTo(BeZero(), `{"FailReason": "Invalid binding, missing port"}`)
	return host, port
}
