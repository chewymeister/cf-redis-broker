package redis_test

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/pivotal-cf/cf-redis-broker/redis"
	"github.com/pivotal-cf/cf-redis-broker/system"
	"github.com/pivotal-golang/lager/lagertest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type fakeProcessChecker struct {
	alive          bool
	lastCheckedPid int
}

func (fakeProcessChecker *fakeProcessChecker) Alive(pid int) bool {
	fakeProcessChecker.lastCheckedPid = pid
	return fakeProcessChecker.alive
}

type fakeProcessKiller struct {
	killed        bool
	lastPidKilled int
}

func (fakeProcessKiller *fakeProcessKiller) Kill(pid int) error {
	fakeProcessKiller.lastPidKilled = pid
	fakeProcessKiller.killed = true
	return nil
}

type fakeInstanceInformer struct{}

func (*fakeInstanceInformer) InstancePid(instanceId string) (int, error) {
	return 123, nil
}

var _ = Describe("Redis Process Controller", func() {
	var processController *redis.OSProcessController
	var instance *redis.Instance = &redis.Instance{}
	var instanceInformer *fakeInstanceInformer
	var logger *lagertest.TestLogger
	var fakeProcessChecker *fakeProcessChecker = &fakeProcessChecker{}
	var fakeProcessKiller *fakeProcessKiller = &fakeProcessKiller{}
	var commandRunner *system.FakeCommandRunner
	var connectionTimeoutErr error

	BeforeEach(func() {
		connectionTimeoutErr = nil
		instanceInformer = &fakeInstanceInformer{}
		logger = lagertest.NewTestLogger("process-controller")
		commandRunner = &system.FakeCommandRunner{}
	})

	JustBeforeEach(func() {
		processController = &redis.OSProcessController{
			Logger:           logger,
			InstanceInformer: instanceInformer,
			CommandRunner:    commandRunner,
			ProcessChecker:   fakeProcessChecker,
			ProcessKiller:    fakeProcessKiller,
			WaitUntilConnectableFunc: func(*net.TCPAddr, time.Duration) error {
				return connectionTimeoutErr
			},
		}
	})

	itStartsARedisProcess := func(executablePath string) {
		Ω(commandRunner.Commands).To(Equal([]string{
			fmt.Sprintf("%s configFilePath --pidfile pidFilePath --dir instanceDataDir --logfile logFilePath", executablePath),
		}))
	}

	Describe("StartAndWaitUntilReady", func() {
		It("runs the right command to start redis", func() {
			processController.StartAndWaitUntilReady(instance, "configFilePath", "instanceDataDir", "pidFilePath", "logFilePath", time.Second*1)
			itStartsARedisProcess("redis-server")
		})

		It("returns no error", func() {
			err := processController.StartAndWaitUntilReady(instance, "", "", "", "", time.Second*1)
			Ω(err).NotTo(HaveOccurred())
		})

		Context("when the redis process fails to start", func() {
			BeforeEach(func() {
				connectionTimeoutErr = errors.New("oops")
			})

			It("returns the same error that the WaitUntilConnectableFunc returns", func() {
				err := processController.StartAndWaitUntilReady(instance, "", "", "", "", time.Second*1)
				Ω(err).To(Equal(connectionTimeoutErr))
			})
		})
	})

	Describe("StartAndWaitUntilReadyWithConfig", func() {

		Context("When using a custom redis-server executable", func() {
			It("runs the right command to start redis", func() {
				processController.RedisServerExecutablePath = "custom/path/to/redis"

				args := []string{
					"configFilePath",
					"--pidfile", "pidFilePath",
					"--dir", "instanceDataDir",
					"--logfile", "logFilePath",
				}
				processController.StartAndWaitUntilReadyWithConfig(instance, args, time.Second*1)
				itStartsARedisProcess("custom/path/to/redis")
			})
		})

		It("runs the right command to start redis", func() {
			args := []string{
				"configFilePath",
				"--pidfile", "pidFilePath",
				"--dir", "instanceDataDir",
				"--logfile", "logFilePath",
			}
			processController.StartAndWaitUntilReadyWithConfig(instance, args, time.Second*1)
			itStartsARedisProcess("redis-server")
		})

		It("returns no error", func() {
			err := processController.StartAndWaitUntilReadyWithConfig(instance, []string{}, time.Second*1)
			Ω(err).NotTo(HaveOccurred())
		})

		Context("when the redis process fails to start", func() {
			BeforeEach(func() {
				connectionTimeoutErr = errors.New("oops")
			})

			It("returns the same error that the WaitUntilConnectableFunc returns", func() {
				err := processController.StartAndWaitUntilReadyWithConfig(instance, []string{}, time.Second*1)
				Ω(err).To(Equal(connectionTimeoutErr))
			})
		})
	})

	Describe("Kill", func() {
		It("kills the correct process", func() {
			err := processController.Kill(instance)
			Ω(err).NotTo(HaveOccurred())

			Ω(fakeProcessKiller.killed).Should(BeTrue())
			Ω(fakeProcessKiller.lastPidKilled).Should(Equal(123))
		})
	})

	Describe("EnsureRunning", func() {
		Context("if the process is already running", func() {
			BeforeEach(func() {
				fakeProcessChecker.alive = true
			})

			It("does not raise an error", func() {
				err := processController.EnsureRunning(instance, "", "", "", "")
				Ω(err).NotTo(HaveOccurred())

				Ω(err).ShouldNot(HaveOccurred())
				Ω(fakeProcessChecker.lastCheckedPid).Should(Equal(123))
			})
		})

		Context("if the process is not already running", func() {
			BeforeEach(func() {
				fakeProcessChecker.alive = false
			})

			It("starts it", func() {
				err := processController.EnsureRunning(instance, "configFilePath", "instanceDataDir", "pidFilePath", "logFilePath")
				Ω(err).ShouldNot(HaveOccurred())

				itStartsARedisProcess("redis-server")
			})

			Context("and it can not be started", func() {
				BeforeEach(func() {
					commandRunner.RunError = errors.New("run error")
				})

				It("should return error", func() {
					err := processController.EnsureRunning(instance, "", "", "", "")
					Ω(err).Should(HaveOccurred())
				})
			})
		})
	})
})
