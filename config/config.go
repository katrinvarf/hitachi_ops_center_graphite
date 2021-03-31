package config

import(
	"io/ioutil"
	"gopkg.in/yaml.v2"
	"fmt"
	"github.com/sirupsen/logrus"
)

//Типы данных для основного конфигурационного файла
type TGeneral struct {
	Graphite TGraphite `yaml:"graphite"`
	Api TApiTuningManager `yaml:"tm_api"`
	Loggers []TLogging `yaml:"logging"`
	Storages []TStorage `yaml:"storages"`
}

type TGraphite struct {
	Host string `yaml:"host"`
	Port string `yaml:"port"`
}

type TApiTuningManager struct {
	Host string `yaml:"host"`
	Port string `yaml:"port"`
	Protocol string `yaml:"proto"`
	User string `yaml:"user"`
	Password string `yaml:"password"`
}

type TStorage struct {
	Serial_Num string `yaml:"serialNumber"`
	Type string `yaml:"type"`
	Name string `yaml:"visibleName"`
}

type TLogging struct {
	LoggerName string `yaml:"logger"`
	File string `yaml:"file"`
	Level string `yaml:"level"`
	Encoding string `yaml:"encoding"`
}

//Типы данных для файла с метриками
type TResourceGroups struct {
	Resources []TResource `yaml:"resources"`
}

type TResource struct {
	Name string `yaml:"name"`
	Label string `yaml:"label"`
	Target string `yaml:"target"`
	Type string `yaml:"type"`
	Branch string `yaml:"branch"`
	Interval int64 `yaml:"interval"`
}

var General = TGeneral{}

func GetConfig(configPath string) (err error){
	var buff []byte
	buff, err = ioutil.ReadFile(configPath)
	if err!=nil{
		fmt.Println("Failed to read general config", err)
		return
	}
	err = yaml.Unmarshal(buff, &General)
	if err!=nil{
		fmt.Println("Failed to decode document", err)
		return
	}
	return nil
}

var ResourceGroups = TResourceGroups{}

func GetResourceConfig(log *logrus.Logger, path string)(err error){
	var buff []byte
	buff, err = ioutil.ReadFile(path)
	if err!=nil{
		log.Warning("Failed to read resource config: Error: ", err)
	}
	err = yaml.Unmarshal(buff, &ResourceGroups)
	if err!=nil{
		log.Warning("Failed to decode document: Error: ", err)
		return
	}
	return nil
}
