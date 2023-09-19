package common

import (
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type Config struct {
	Listen           string    `yaml:"listen"`
	Log              LogConfig `yaml:"log"`
	DynamicClient    *dynamic.DynamicClient
	KubernetesClient *kubernetes.Clientset
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}
