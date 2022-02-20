package pkg

import (
	"fmt"
	"io"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/api/apps/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/klog"
	"net/http"
	"strings"
)

var (
	runtimeScheme = runtime.NewScheme()
	codeFactory   = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codeFactory.UniversalDeserializer()
)

const (
	AnnotationMutateKey = "skywalking-agent-injection" // io.ydzs.admission-registry/mutate=no/off/false/n
	AnnotationStatusKey = "io.ydzs.admission-registry/status" // io.ydzs.admission-registry/status=mutated
)

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}


type WhSvrParam struct {
	Port     int
	CertFile string
	KeyFile  string
}

type WebhookServer struct {
	Server              *http.Server // http server
	WhiteListRegistries []string     // 白名单的镜像仓库列表
}

func (s *WebhookServer) Handler(w http.ResponseWriter, r *http.Request) {
	var body []byte
	// 判断接收到请求体是否为为空
	if r.Body != nil {
		if data, err := io.ReadAll(r.Body); err == nil {
			body = data
		}
	}


	// 判断body长度是否0
	if len(body) == 0 {
		http.Error(w, "no body found", http.StatusBadRequest)
		return
	}
	// 校验内容类型是否正确
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		http.Error(w, "invalid Content-Type, want `application/json`", http.StatusUnsupportedMediaType)
		return
	}
	var admissionResponse *admissionv1.AdmissionResponse
	requestedAdmissionReview := admissionv1.AdmissionReview{}

	if _, _, err := deserializer.Decode(body, nil, &requestedAdmissionReview); err != nil {
		klog.Errorf("Can't decode body: %v", err)
		admissionResponse = &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Code:    http.StatusInternalServerError,
				Message: err.Error(),
			},
		}
	} else {
		// 序列化成功，也就是说获取到了请求的 AdmissionReview 的数据
		if r.URL.Path == "/mutate" {
			admissionResponse = s.mutate(&requestedAdmissionReview)
		}
	}
	// 数据序列化（validate、mutate）请求的数据都是 AdmissionReview
	responseAdmissionReview := admissionv1.AdmissionReview{}
	// admission/v1
	responseAdmissionReview.APIVersion = requestedAdmissionReview.APIVersion
	responseAdmissionReview.Kind = requestedAdmissionReview.Kind
	if admissionResponse != nil {
		responseAdmissionReview.Response = admissionResponse
		if requestedAdmissionReview.Request != nil { // 返回相同的 UID
			responseAdmissionReview.Response.UID = requestedAdmissionReview.Request.UID
		}
}}


func (s *WebhookServer) mutate(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	req := ar.Request

	var (
		objectMeta *metav1.ObjectMeta
		deployment v1beta1.Deployment
	)
	klog.Infof("AdmissionReview for Kind=%s, Namespace=%s Name=%s UID=%s",
		req.Kind.Kind, req.Namespace, req.Name, req.UID)

	// 序列化相应的对象
	switch req.Kind.Kind {
	case "Deployment":
		// 实例化Pod对象
		if err := json.Unmarshal(req.Object.Raw,&deployment);
		err != nil {
			klog.Errorf("Can't not unmarshal raw object: %v", err)
			return &admissionv1.AdmissionResponse{
				Result: &metav1.Status{
					Code:    http.StatusBadRequest,
					Message: err.Error(),
				},
			}
		}
		objectMeta = &deployment.ObjectMeta

	default:
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Code:    http.StatusBadRequest,
				Message: fmt.Sprintf("Can't handle the kind(%s) object", req.Kind.Kind),
			},
		}
	}

	// 判断是否需要真的执行 mutate 操作
	if !mutationRequired(objectMeta) {
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	// 需要执行 mutate 操作
	//annotations := map[string]string{
	//	AnnotationStatusKey: "mutated",
	//}
	var patch []patchOperation
	patch = append(patch, mutateAnnotations(deployment)...)

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		klog.Errorf("patch marshal error: %v", err)
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Code:    http.StatusBadRequest,
				Message: err.Error(),
			},
		}
	}

	return &admissionv1.AdmissionResponse{
		Allowed: true,
		Patch:   patchBytes,
		PatchType: func() *admissionv1.PatchType {
			pt := admissionv1.PatchTypeJSONPatch
			return &pt
		}(),
	}
}


// 判断是否需要执行mutation操作
func mutationRequired(metadata *metav1.ObjectMeta) bool {
	// 获取注解赋给变量
	annotations := metadata.GetLabels()
	// 判断注解是否为空
	if annotations == nil {
		annotations = map[string]string{}
	}

	var required bool

	// 判断注解
	switch strings.ToLower(annotations[AnnotationMutateKey]) {
	case "enabled":
		required = true
	default:
		required = false
	}
	klog.Infof("Mutation policy for %s/%s: required: %v", metadata.Name, metadata.Namespace, required)
	return required
}

// 修改操作
func mutateAnnotations(target v1beta1.Deployment) (patch []patchOperation) {
	klog.Info("获取变量")
	for i := range target.Spec.Template.Spec.Containers{
		for i := range target.Spec.Template.Spec.Containers[i].Env{
			if target.Spec.Template.Spec.Containers[i].Env[i].Name == "JAVA_OPTS"{
				klog.Info("已获取到java变量")
				klog.Info(target.Spec.Template.Spec.Containers[i].Env[i].Value)
				key := target.Spec.Template.Spec.Containers[i].Env[i].Name
				Env := target.Spec.Template.Spec.Containers[i].Env[i].Value
				Env = Env + " -javaagent:/usr/agent/skywalking-agent.jar -Dskywalking.agent.namespace=uat -Dskywalking.agent.service_name=fp-android-transit-job -Dskywalking.collector.backend_service=10.0.54.104:5006"
				patch = append(patch, patchOperation{
					Op: "replace",
					Path: "/spec/template/spec/containers/env/" + key,
					Value: Env,
				})
			}
		}
	}
	//for key, value := range added {
	//	if target == nil || target[key] == "" {
	//		target = map[string]string{}
	//		patch = append(patch, patchOperation{
	//			Op:   "add",
	//			Path: "/metadata/annotations",
	//			Value: map[string]string{
	//				key: value,
	//			},
	//		})
	//	} else {
	//		patch = append(patch, patchOperation{
	//			Op:    "replace",
	//			Path:  "/metadata/annotations/" + key,
	//			Value: value,
	//		})
	//	}
	//}

	return
}
