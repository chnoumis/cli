package describer

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/kubedb/cli/pkg/printer"
	"github.com/kubedb/cli/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubernetes/pkg/apis/apps"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/extensions"
	coreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	"k8s.io/kubernetes/pkg/printers"
)

func (d *humanReadableDescriber) describeStatefulSet(statefulSet apps.StatefulSet, out io.Writer) {
	clientSet, err := d.ClientSet()
	if err != nil {
		return
	}

	pc := clientSet.Core().Pods(statefulSet.Namespace)

	selector, err := metav1.LabelSelectorAsSelector(statefulSet.Spec.Selector)
	if err != nil {
		return
	}

	running, waiting, succeeded, failed, err := getPodStatusForController(pc, selector)
	if err != nil {
		return
	}

	fmt.Fprint(out, "\n")
	fmt.Fprint(out, "StatefulSet:\t\n")
	fmt.Fprintf(out, "  Name:\t%s\n", statefulSet.Name)
	fmt.Fprintf(out, "  Replicas:\t%d current / %d desired\n", statefulSet.Status.Replicas, statefulSet.Spec.Replicas)
	fmt.Fprintf(out, "  CreationTimestamp:\t%s\n", timeToString(&statefulSet.CreationTimestamp))
	fmt.Fprintf(out, "  Pods Status:\t%d Running / %d Waiting / %d Succeeded / %d Failed\n", running, waiting, succeeded, failed)
}

func (d *humanReadableDescriber) describeDeployments(deployement extensions.Deployment, out io.Writer) {
	clientSet, err := d.ClientSet()
	if err != nil {
		return
	}

	pc := clientSet.Core().Pods(deployement.Namespace)
	selector, err := metav1.LabelSelectorAsSelector(deployement.Spec.Selector)
	if err != nil {
		return
	}

	running, waiting, succeeded, failed, err := getPodStatusForController(pc, selector)
	if err != nil {
		return
	}

	fmt.Fprint(out, "\n")
	fmt.Fprint(out, "Deployment:\t\n")
	fmt.Fprintf(out, "  Name:\t%s\n", deployement.Name)
	fmt.Fprintf(out, "  Replicas:\t%d current / %d desired\n", deployement.Status.Replicas, deployement.Spec.Replicas)
	fmt.Fprintf(out, "  CreationTimestamp:\t%s\n", timeToString(&deployement.CreationTimestamp))
	fmt.Fprintf(out, "  Pods Status:\t%d Running / %d Waiting / %d Succeeded / %d Failed\n", running, waiting, succeeded, failed)
}

func getPodStatusForController(c coreclient.PodInterface, selector labels.Selector) (running, waiting, succeeded, failed int, err error) {
	options := metav1.ListOptions{LabelSelector: selector.String()}
	rcPods, err := c.List(options)
	if err != nil {
		return
	}
	for _, pod := range rcPods.Items {
		switch pod.Status.Phase {
		case kapi.PodRunning:
			running++
		case kapi.PodPending:
			waiting++
		case kapi.PodSucceeded:
			succeeded++
		case kapi.PodFailed:
			failed++
		}
	}
	return
}

func (d *humanReadableDescriber) describeService(service kapi.Service, out io.Writer) {
	fmt.Fprint(out, "\n")
	fmt.Fprint(out, "Service:\t\n")
	fmt.Fprintf(out, "  Name:\t%s\n", service.Name)
	fmt.Fprintf(out, "  Type:\t%s\n", service.Spec.Type)
	fmt.Fprintf(out, "  IP:\t%s\n", service.Spec.ClusterIP)
	if len(service.Spec.ExternalIPs) > 0 {
		fmt.Fprintf(out, "  External IPs:\t%v\n", strings.Join(service.Spec.ExternalIPs, ","))
	}
	if service.Spec.ExternalName != "" {
		fmt.Fprintf(out, "  External Name:\t%s\n", service.Spec.ExternalName)
	}
	if len(service.Status.LoadBalancer.Ingress) > 0 {
		list := buildIngressString(service.Status.LoadBalancer.Ingress)
		fmt.Fprintf(out, "  LoadBalancer Ingress:\t%s\n", list)
	}

	for i := range service.Spec.Ports {
		sp := &service.Spec.Ports[i]

		name := sp.Name
		if name == "" {
			name = "<unset>"
		}
		fmt.Fprintf(out, "  Port:\t%s\t%d/%s\n", name, sp.Port, sp.Protocol)
		if sp.NodePort != 0 {
			fmt.Fprintf(out, "  NodePort:\t%s\t%d/%s\n", name, sp.NodePort, sp.Protocol)
		}
	}
}

func buildIngressString(ingress []kapi.LoadBalancerIngress) string {
	var buffer bytes.Buffer

	for i := range ingress {
		if i != 0 {
			buffer.WriteString(", ")
		}
		if ingress[i].IP != "" {
			buffer.WriteString(ingress[i].IP)
		} else {
			buffer.WriteString(ingress[i].Hostname)
		}
	}
	return buffer.String()
}

func (d *humanReadableDescriber) describeSecret(namespace, name string, prefix string, out io.Writer) {
	clientSet, err := d.ClientSet()
	if err != nil {
		return
	}

	c := clientSet.Core().Secrets(namespace)

	secret, err := c.Get(name, metav1.GetOptions{})
	if err != nil {
		return
	}

	fmt.Fprint(out, "\n")
	if prefix == "" {
		fmt.Fprint(out, "Secret:\n")
	} else {
		fmt.Fprintf(out, "%s Secret:\n", prefix)
	}
	fmt.Fprintf(out, "  Name:\t%s\n", secret.Name)
	fmt.Fprintf(out, "  Type:\t%s\n", secret.Type)
	fmt.Fprint(out, "  Data\n")
	fmt.Fprint(out, "  ====\n")
	for k, v := range secret.Data {
		fmt.Fprintf(out, "  %s:\t%d bytes\n", k, len(v))
	}
}

func describeEvents(el *kapi.EventList, out io.Writer) {
	fmt.Fprint(out, "\n")
	if len(el.Items) == 0 {
		fmt.Fprint(out, "No events.\n")
		return
	}

	sort.Sort(util.SortableEvents(el.Items))

	fmt.Fprint(out, "Events:\n")
	w := printers.GetNewTabWriter(out)

	fmt.Fprint(w, "  FirstSeen\tLastSeen\tCount\tFrom\tType\tReason\tMessage\n")
	fmt.Fprint(w, "  ---------\t--------\t-----\t----\t--------\t------\t-------\n")
	for _, e := range el.Items {
		fmt.Fprintf(w, "  %s\t%s\t%d\t%v\t%v\t%v\t%v\n",
			printer.TranslateTimestamp(e.FirstTimestamp),
			printer.TranslateTimestamp(e.LastTimestamp),
			e.Count,
			e.Source.Component,
			e.Type,
			e.Reason,
			e.Message)
	}
	w.Flush()
}
