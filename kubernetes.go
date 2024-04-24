package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var defaultNamespace = "default"

type ConfigMapConfigManager struct {
	Client    kubernetes.Interface
	Namespace string
}

func (kcm ConfigMapConfigManager) Get(key string) ([]byte, error) {
	name, configKey, err := parsePath(key)
	if err != nil {
		return nil, err
	}

	configMap, err := kcm.Client.CoreV1().ConfigMaps(kcm.Namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return extractKeyFromConfigMap(configMap, configKey)
}

func (kcm ConfigMapConfigManager) Watch(key string, stop chan bool) <-chan *Response {
	name, configKey, err := parsePath(key)
	if err != nil {
		return nil
	}

	resp := make(chan *Response)

	go func(configMapName string, key string, stop <-chan bool, resp chan *Response) {
		defer close(resp)

		watch, err := kcm.Client.CoreV1().ConfigMaps(kcm.Namespace).Watch(context.Background(), metav1.ListOptions{})
		if err != nil {
			resp <- &Response{Error: err}
		}

		for {
			select {
			case <-stop:
				watch.Stop()
				return
			case event := <-watch.ResultChan():
				configMap := event.Object.(*v1.ConfigMap)
				if configMap.Name == configMapName {
					if data, err := extractKeyFromConfigMap(configMap, key); err != nil {
						resp <- &Response{Error: err}
					} else {
						resp <- &Response{Value: data}
					}
				}
			}
		}

	}(name, configKey, stop, resp)

	return resp
}

func extractKeyFromConfigMap(configMap *v1.ConfigMap, key string) ([]byte, error) {
	if val, ok := configMap.Data[key]; ok {
		return []byte(val), nil
	}

	if val, ok := configMap.BinaryData[key]; ok {
		return val, nil
	}

	return nil, errors.New("missing file in config map")
}

type SecretConfigManager struct {
	Client    kubernetes.Interface
	Namespace string
}

func (scm SecretConfigManager) Get(key string) ([]byte, error) {
	name, configKey, err := parsePath(key)
	if err != nil {
		return nil, err
	}
	secret, err := scm.Client.CoreV1().Secrets(scm.Namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if val, ok := secret.Data[configKey]; ok {
		return []byte(val), nil
	}

	return nil, errors.New("missing file in secret")
}

func (scm SecretConfigManager) Watch(key string, stop chan bool) <-chan *Response {
	name, configKey, err := parsePath(key)
	if err != nil {
		return nil
	}

	resp := make(chan *Response)

	go func(secretName string, key string, stop <-chan bool, resp chan *Response) {
		defer close(resp)

		watch, err := scm.Client.CoreV1().Secrets(scm.Namespace).Watch(context.Background(), metav1.ListOptions{})
		if err != nil {
			resp <- &Response{Error: err}
		}

		for {
			select {
			case <-stop:
				watch.Stop()
				return
			case event := <-watch.ResultChan():
				secret := event.Object.(*v1.Secret)
				if secret.Name == secretName {
					if val, ok := secret.Data[configKey]; ok {
						resp <- &Response{Value: val}
					} else {
						resp <- &Response{Error: errors.New("missing file in secret")}
					}
				}
			}
		}

	}(name, configKey, stop, resp)

	return resp
}

func parsePath(path string) (name string, key string, err error) {
	values := strings.Split(path, "/")
	switch len(values) {
	case 2:
		name = values[0]
		key = values[1]
	default:
		err = fmt.Errorf("unstructual path, path should be cm(secrete)name/key")
	}
	return
}

func NewConfigMapConfigManager(configPath string, namespace string) (ConfigManager, error) {
	config, err := GetConfigFromReader(configPath)
	if err != nil {
		return nil, err
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	if namespace == "" {
		namespace = defaultNamespace
	}
	return &ConfigMapConfigManager{
		Client:    clientset,
		Namespace: namespace,
	}, nil
}

func NewSecretConfigManager(configPath string, namespace string) (ConfigManager, error) {
	config, err := GetConfigFromReader(configPath)
	if err != nil {
		return nil, err
	}
	// creates the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &SecretConfigManager{
		Client:    clientset,
		Namespace: namespace,
	}, nil
}

func GetConfigFromReader(configPath string) (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// if you want to change the loading rules (which files in which order), you can do so here
	if configPath != "" {
		loadingRules.Precedence = append([]string{configPath}, loadingRules.Precedence...)
	}

	configOverrides := &clientcmd.ConfigOverrides{}
	// if you want to change override values or bind them to flags, there are methods to help you

	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	return kubeConfig.ClientConfig()
}
