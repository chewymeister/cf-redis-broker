package redis

import (
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pivotal-cf/cf-redis-broker/broker"
	"github.com/pivotal-cf/cf-redis-broker/brokerconfig"
	"github.com/pivotal-cf/cf-redis-broker/redisconf"
	"github.com/pivotal-golang/lager"
)

type LocalRepository struct {
	RedisConf brokerconfig.ServiceConfiguration
	Logger    lager.Logger
}

func NewLocalRepository(redisConf brokerconfig.ServiceConfiguration, logger lager.Logger) *LocalRepository {
	return &LocalRepository{
		RedisConf: redisConf,
		Logger:    logger,
	}
}

func (repo *LocalRepository) FindByID(instanceID string) (*Instance, error) {
	conf, err := redisconf.Load(repo.InstanceConfigPath(instanceID))
	if err != nil {
		return nil, err
	}

	port, err := strconv.Atoi(conf.Get("port"))
	if err != nil {
		return nil, err
	}

	instance := &Instance{
		ID:       instanceID,
		Password: conf.Get("requirepass"),
		Port:     port,
		Host:     repo.RedisConf.Host,
	}

	return instance, nil
}

func (repo *LocalRepository) InstanceExists(instanceID string) (bool, error) {
	if _, err := os.Stat(repo.InstanceBaseDir(instanceID)); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

// Eventually: make lock the first thing to be called
// EnsureDirectoriesExist -> EnsureLogDirectoryExists

func (repo *LocalRepository) Setup(instance *Instance) error {
	err := repo.EnsureDirectoriesExist(instance)
	if err != nil {
		repo.Logger.Error("ensure-dirs-exist", err, lager.Data{
			"instance_id": instance.ID,
		})
		return err
	}

	err = repo.Lock(instance)
	if err != nil {
		repo.Logger.Error("lock-shared-instance", err, lager.Data{
			"instance_id": instance.ID,
		})
		return err
	}

	err = repo.WriteConfigFile(instance)
	if err != nil {
		repo.Logger.Error("write-config-file", err, lager.Data{
			"instance_id": instance.ID,
		})
		return err
	}

	return nil
}

func (repo *LocalRepository) Lock(instance *Instance) error {
	lockFilePath := repo.lockFilePath(instance)
	lockFile, err := os.Create(lockFilePath)
	if err != nil {
		return err
	}
	lockFile.Close()

	return nil
}

func (repo *LocalRepository) Unlock(instance *Instance) error {
	lockFilePath := repo.lockFilePath(instance)
	err := os.Remove(lockFilePath)
	if err != nil {
		return err
	}

	return nil
}

func (repo *LocalRepository) lockFilePath(instance *Instance) string {
	return filepath.Join(repo.InstanceBaseDir(instance.ID), "lock")
}

func (repo *LocalRepository) AllInstances() ([]*Instance, error) {
	instances := []*Instance{}

	instanceDirs, err := ioutil.ReadDir(repo.RedisConf.InstanceDataDirectory)
	if err != nil {
		return instances, err
	}

	for _, instanceDir := range instanceDirs {

		instance, err := repo.FindByID(instanceDir.Name())

		if err != nil {
			return instances, err
		}

		instances = append(instances, instance)
	}

	return instances, nil
}

func (repo *LocalRepository) InstanceCount() (int, error) {
	instances, err := repo.AllInstances()
	return len(instances), err
}

func (repo *LocalRepository) Bind(instanceID string, bindingID string) (broker.InstanceCredentials, error) {
	instance, err := repo.FindByID(instanceID)
	if err != nil {
		return broker.InstanceCredentials{}, err
	}
	return broker.InstanceCredentials{
		Host:     instance.Host,
		Port:     instance.Port,
		Password: instance.Password,
	}, nil
}

func (repo *LocalRepository) Unbind(instanceID string, bindingID string) error {
	return nil
}

func (repo *LocalRepository) Delete(instanceID string) error {
	err := os.RemoveAll(repo.InstanceBaseDir(instanceID))
	if err != nil {
		return err
	}

	err = os.RemoveAll(repo.InstanceLogDir(instanceID))
	if err != nil {
		return err
	}

	return nil
}

func (repo *LocalRepository) EnsureDirectoriesExist(instance *Instance) error {
	err := os.MkdirAll(repo.InstanceDataDir(instance.ID), 0755)
	if err != nil {
		return err
	}

	err = os.MkdirAll(repo.InstanceLogDir(instance.ID), 0755)
	if err != nil {
		return err
	}

	return nil
}

func (repo *LocalRepository) WriteConfigFile(instance *Instance) error {
	return redisconf.CopyWithInstanceAdditions(
		repo.RedisConf.DefaultConfigPath,
		repo.InstanceConfigPath(instance.ID),
		instance.ID,
		strconv.Itoa(instance.Port),
		instance.Password,
	)
}

func (repo *LocalRepository) InstanceBaseDir(instanceID string) string {
	return path.Join(repo.RedisConf.InstanceDataDirectory, instanceID)
}

func (repo *LocalRepository) InstanceDataDir(instanceID string) string {
	InstanceBaseDir := repo.InstanceBaseDir(instanceID)
	return path.Join(InstanceBaseDir, "db")
}

func (repo *LocalRepository) InstanceLogDir(instanceID string) string {
	return path.Join(repo.RedisConf.InstanceLogDirectory, instanceID)
}

func (repo *LocalRepository) InstanceLogFilePath(instanceID string) string {
	return path.Join(repo.InstanceLogDir(instanceID), "redis-server.log")
}

func (repo *LocalRepository) InstanceConfigPath(instanceID string) string {
	return path.Join(repo.InstanceBaseDir(instanceID), "redis.conf")
}

func (repo *LocalRepository) InstancePidFilePath(instanceID string) string {
	return path.Join(repo.InstanceBaseDir(instanceID), "redis-server.pid")
}

func (repo *LocalRepository) InstancePid(instanceID string) (pid int, err error) {
	pidFilePath := repo.InstancePidFilePath(instanceID)

	fileContent, pidFileErr := ioutil.ReadFile(pidFilePath)
	if pidFileErr != nil {
		return pid, pidFileErr
	}

	pidValue := strings.TrimSpace(string(fileContent))

	parsedPid, parseErr := strconv.ParseInt(pidValue, 10, 32)
	if parseErr != nil {
		return pid, parseErr
	}

	return int(parsedPid), err
}
