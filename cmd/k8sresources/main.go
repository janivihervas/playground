package main

import (
	"flag"
	"fmt"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"
)

type resource struct {
	milliCPU int64
	mem      int64
}

func getConfig() string {
	var (
		kubeconfig string
		configEnv = os.Getenv("KUBECONFIG")
		configFlag string
	)

	flag.StringVar(&configFlag, "kubeconfig", "", "absolute path to the kubeconfig file")
	flag.Parse()

	if configFlag != "" {
		kubeconfig = configFlag
	} else if configEnv != "" {
		kubeconfig = configEnv
	} else if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	return kubeconfig
}

func main() {
	kubeconfig := getConfig()
	if kubeconfig == "" {
		fmt.Println("kubeconfig is empty. Either set it as a flag or with KUBECONFIG environment variable")
		flag.PrintDefaults()
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	namespaceList, err := clientset.CoreV1().Namespaces().List(v1.ListOptions{
		Limit: 100,
	})
	if err != nil {
		panic(err)
	}

	resources := make(map[string]resource, len(namespaceList.Items))
	var namespaces []string

 for _, ns := range namespaceList.Items {
		resources[ns.Name] = resource{}
		namespaces = append(namespaces, ns.Name)


		pods, err := clientset.CoreV1().Pods(ns.Name).List(v1.ListOptions{
			FieldSelector: "status.phase=Running",
		})
		if err != nil {
			panic(err)
		}

		for _, pod := range pods.Items {
			for _, container := range pod.Spec.Containers {
				var (
					cpu = resources[ns.Name].milliCPU
					mem = resources[ns.Name].mem
					cpuResource = container.Resources.Requests.Cpu()
					memResource = container.Resources.Requests.Memory()
				)

				if cpuResource != nil {
					cpu += cpuResource.MilliValue()
				}
				if memResource != nil {
					mem += memResource.Value()
				}

				resources[ns.Name] = resource{
					milliCPU: cpu,
					mem:      mem,
				}
			}
		}
	}

	var (
		w = tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', tabwriter.TabIndent)
		totalCpu int64
		totalMem int64
	)

	_, _ = fmt.Fprintln(w, "NAMESPACE\tCPU REQUESTS (vCPU)\tMEMORY REQUESTS (MB)")

	sort.Strings(namespaces)
	for _, ns := range namespaces {
		_, _ = fmt.Fprintf(w, "%s\t%0.3f\t%0.f\n", ns, float64(resources[ns].milliCPU) / 1000, float64(resources[ns].mem) / (1024*1024))

		totalCpu += resources[ns].milliCPU
		totalMem += resources[ns].mem
	}

	_, _ = fmt.Fprintln(w, "\t\t")
	_, _ = fmt.Fprintf(w, "TOTAL\t%0.3f vCPU\t%0.f MB\n", float64(totalCpu) / 1000, float64(totalMem) / (1024*1024))
	_ = w.Flush()
}

