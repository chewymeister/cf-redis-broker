package brokerintegration_test

import (
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/pborman/uuid"
	"github.com/pivotal-cf/cf-redis-broker/integration/helpers"
)

var _ = Describe("Provision shared instance", func() {

	var instanceID string
	var initialRedisProcessCount int

	BeforeEach(func() {
		instanceID = uuid.NewRandom().String()
		initialRedisProcessCount = getRedisProcessCount()
	})

	AfterEach(func() {
		Ω(getRedisProcessCount()).To(Equal(initialRedisProcessCount))
	})

	Context("when instance is created successfully", func() {
		AfterEach(func() {
			brokerClient.DeprovisionInstance(instanceID)
		})

		It("returns 201", func() {
			status, _ := brokerClient.ProvisionInstance(instanceID, "shared")
			Expect(status).To(Equal(201))
		})

		It("returns empty JSON", func() {
			_, body := brokerClient.ProvisionInstance(instanceID, "shared")
			Expect(body).To(MatchJSON("{}"))
		})

		It("starts a Redis instance", func() {
			brokerClient.ProvisionInstance(instanceID, "shared")
			Ω(getRedisProcessCount()).To(Equal(initialRedisProcessCount + 1))
		})

		It("writes a Redis config to the instance directory", func() {
			brokerClient.ProvisionInstance(instanceID, "shared")
			configPath := filepath.Join(brokerConfig.RedisConfiguration.InstanceDataDirectory, instanceID, "redis.conf")
			_, err := os.Stat(configPath)
			Ω(err).NotTo(HaveOccurred())
		})

		It("the redis instance logs to the right file", func() {
			brokerClient.ProvisionInstance(instanceID, "shared")

			logFilePath := filepath.Join(brokerConfig.RedisConfiguration.InstanceLogDirectory, instanceID, "redis-server.log")
			_, err := os.Stat(logFilePath)
			Ω(err).NotTo(HaveOccurred())

			logBytes, err := ioutil.ReadFile(logFilePath)
			Ω(err).NotTo(HaveOccurred())

			logFile := string(logBytes)
			Ω(logFile).Should(ContainSubstring("Server started"))
		})
	})

	Context("when the service instance limit has been met", func() {
		BeforeEach(func() {
			brokerClient.ProvisionInstance("1", "shared")
			brokerClient.ProvisionInstance("2", "shared")
			brokerClient.ProvisionInstance("3", "shared")
		})

		AfterEach(func() {
			brokerClient.DeprovisionInstance("1")
			brokerClient.DeprovisionInstance("2")
			brokerClient.DeprovisionInstance("3")
		})

		It("does not start a Redis instance", func() {
			brokerClient.ProvisionInstance("4", "shared")
			defer brokerClient.DeprovisionInstance("4")
			Ω(getRedisProcessCount()).To(Equal(initialRedisProcessCount + 3))
		})

		It("returns a 500", func() {
			statusCode, _ := brokerClient.ProvisionInstance("4", "shared")
			defer brokerClient.DeprovisionInstance("4")
			Ω(statusCode).To(Equal(500))
		})

		It("returns a useful error message in the correct JSON format", func() {
			_, body := brokerClient.ProvisionInstance("4", "shared")
			defer brokerClient.DeprovisionInstance("4")

			Ω(string(body)).To(MatchJSON(`{"description":"instance limit for this service has been reached"}`))
		})
	})

	Context("when there is an error in instance setup", func() {
		AfterEach(func() {
			err := os.Chmod(helpers.TestDataDir, 0755)
			Expect(err).NotTo(HaveOccurred())
		})

		It("logs the error", func() {
			instanceID := "1"

			err := os.Chmod(helpers.TestDataDir, 0400)
			Expect(err).NotTo(HaveOccurred())
			statusCode, _ := brokerClient.ProvisionInstance(instanceID, "shared")

			Expect(statusCode).To(Equal(500))
			Expect(brokerSession.Buffer()).To(gbytes.Say(`"redis-broker.ensure-dirs-exist"`))
			Expect(brokerSession.Buffer()).To(gbytes.Say(
				`"error":"mkdir ` + helpers.TestDataDir + `/` + instanceID + `: permission denied"`,
			))
		})
	})

	Context("when the service instance already exists", func() {
		BeforeEach(func() {
			brokerClient.ProvisionInstance(instanceID, "shared")
		})

		AfterEach(func() {
			brokerClient.DeprovisionInstance(instanceID)
		})

		It("should fail if we try to provision a second instance with the same ID", func() {
			numRedisProcessesBeforeExec := getRedisProcessCount()
			statusCode, body := brokerClient.ProvisionInstance(instanceID, "shared")
			Ω(statusCode).To(Equal(409))

			Ω(string(body)).To(MatchJSON(`{}`))
			Ω(getRedisProcessCount()).To(Equal(numRedisProcessesBeforeExec))
		})
	})

})
