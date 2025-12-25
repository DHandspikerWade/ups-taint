package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	nut "github.com/robbiet480/go.nut"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const TaintKey = "ups.spikedhand.com/status"
const NodeLabel = "ups.spikedhand.com/name"

type DesiredTaint struct {
	Present bool
	Value   string
	Effect  coreV1.TaintEffect
}

func getTaint(status string, percentage float32) DesiredTaint {
	// TODO: read this from ENV
	var threshold float32 = 20.0

	switch {
		case strings.Contains(status, "OL"):
			return DesiredTaint{Present: false}

		case strings.Contains(status, "OB") && strings.Contains(status, "LB"):
			return DesiredTaint{
				Present: true,
				Value:   "low-battery",
				Effect:  coreV1.TaintEffectNoExecute,
			}
		
		case threshold > 0 && percentage < threshold:
			return DesiredTaint{
				Present: true,
				Value:   "below-threshold",
				Effect:  coreV1.TaintEffectNoExecute,
			}

		case strings.Contains(status, "OB"):
			return DesiredTaint{
				Present: true,
				Value:   "on-battery",
				Effect:  coreV1.TaintEffectNoSchedule,
			}

		default:
			// An unknown state is not a healthy node
			return DesiredTaint{
				Present: true,
				Value:   "unknown",
				Effect:  coreV1.TaintEffectNoSchedule,
			}
	}
}

func computeTaints(
	existing []coreV1.Taint,
	desired DesiredTaint,
) ([]coreV1.Taint, bool) {

	var result []coreV1.Taint
	found := false

	for _, taint := range existing {
		if taint.Key == TaintKey {
			found = true
			if desired.Present {
				result = append(result, coreV1.Taint{
					Key:    TaintKey,
					Value:  desired.Value,
					Effect: desired.Effect,
				})
			}
		} else {
			result = append(result, taint)
		}
	}

	if !found && desired.Present {
		result = append(result, coreV1.Taint{
			Key:    TaintKey,
			Value:  desired.Value,
			Effect: desired.Effect,
		})
	}

	if taintsEqual(existing, result) {
		return nil, false
	}

	return result, true
}

func taintsEqual(a, b []coreV1.Taint) bool {
	if len(a) != len(b) {
		return false
	}

	taintList := map[string]coreV1.Taint{}
	for _, taint := range a {
		taintList[taint.Key] = taint
	}

	for _, taint := range b {
		existing, ok := taintList[taint.Key];

		if !ok || existing.Value != taint.Value || existing.Effect != taint.Effect {
			return false
		}
	}
	return true
}

func updateTaints(
	ctx context.Context,
	client kubernetes.Interface,
	nodeName string,
	taints []coreV1.Taint,
) error {
	payload := map[string]interface{}{
		"spec": map[string]interface{}{
			"taints": taints,
		},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	_, err = client.CoreV1().Nodes().Patch(
		ctx,
		nodeName,
		types.StrategicMergePatchType,
		data,
		metav1.PatchOptions{},
	)

	return err
}

func applyNodeTaint(upsName string, newTaint DesiredTaint) {
	// TODO: unclear why context is needed or it's effect
	ctx, cancel := context.WithTimeout(context.Background(), 30 * time.Second)
	defer cancel()

	var cfg *rest.Config
	var confErr error

	cfg, confErr = rest.InClusterConfig()
	if confErr != nil {
		if confErr == rest.ErrNotInCluster {
			// fallback to local config for testing
			cfg, confErr = clientcmd.BuildConfigFromFlags("", os.Getenv("HOME") + "/.kube/config")
			if (confErr != nil) {
				panic(confErr)
			}
		} else {
			panic(confErr)
		}
	}

	kube, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		panic(err)
	}

	nodes, err := kube.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: NodeLabel + "=" + upsName,
	})

	if err != nil {
		panic(err)
	}

	for _, node := range nodes.Items {
		updatedTaints, changed := computeTaints(node.Spec.Taints, newTaint)
		if !changed {
			continue
		}

		fmt.Printf("Updating taints on %s\n", node.Name)

		err := updateTaints(ctx, kube, node.Name, updatedTaints)
		if err != nil {
			continue
		}
	}
}

func main() {
	nutClient, err := nut.Connect(os.Getenv("NUT_ADDRESS"))

	if err != nil {
		panic(err)
	}

	_, authenticationError := nutClient.Authenticate(os.Getenv("NUT_USERNAME"), os.Getenv("NUT_PASSWORD"))
	if authenticationError != nil {
		panic(authenticationError)
	}

	upsName := os.Getenv("NUT_UPS_NAME")
	if upsName == "" {
		panic("NUT_UPS_NAME must be set")
	}

	upsList, listErr := nutClient.GetUPSList()
	if listErr != nil {
		panic(listErr)
	}

	for _, ups := range upsList {
		upsVars, _ := ups.GetVariables()

		var status string
		var battery float32

		for _, variable := range upsVars {
			if variable.Name == "ups.status" {
				status = variable.Value.(string)
				break
			} else if variable.Name == "battery.charge" {

			}
		}

		if status != "" && battery > 0 {
			applyNodeTaint(ups.Name, getTaint(status, battery))
		}
	}
}
