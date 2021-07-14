package executor

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/kedacore/keda/v2/api/v1alpha1"
	"github.com/kedacore/keda/v2/pkg/mock/mock_client"
	"github.com/kedacore/keda/v2/pkg/mock/mock_scale"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
)

func TestScaleToFallbackReplicasWhenNotActiveAndIsError(t *testing.T) {
	ctrl := gomock.NewController(t)
	client := mock_client.NewMockClient(ctrl)
	recorder := record.NewFakeRecorder(1)
	mockScaleClient := mock_scale.NewMockScalesGetter(ctrl)
	mockScaleInterface := mock_scale.NewMockScaleInterface(ctrl)
	statusWriter := mock_client.NewMockStatusWriter(ctrl)

	scaleExecutor := NewScaleExecutor(client, mockScaleClient, nil, recorder)

	scaledObject := v1alpha1.ScaledObject{
		ObjectMeta: v1.ObjectMeta{
			Name:      "some name",
			Namespace: "some namespace",
		},
		Spec: v1alpha1.ScaledObjectSpec{
			ScaleTargetRef: &v1alpha1.ScaleTarget{
				Name: "some name",
			},
			Fallback: &v1alpha1.Fallback{
				FailureThreshold: 3,
				Replicas:         5,
			},
		},
		Status: v1alpha1.ScaledObjectStatus{
			ScaleTargetGVKR: &v1alpha1.GroupVersionKindResource{
				Group: "apps",
				Kind:  "Deployment",
			},
		},
	}

	scaledObject.Status.Conditions = *v1alpha1.GetInitializedConditions()

	numberOfReplicas := int32(2)

	client.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).SetArg(2, appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Replicas: &numberOfReplicas,
		},
	})

	scale := &autoscalingv1.Scale{
		Spec: autoscalingv1.ScaleSpec{
			Replicas: numberOfReplicas,
		},
	}

	mockScaleClient.EXPECT().Scales(gomock.Any()).Return(mockScaleInterface).Times(2)
	mockScaleInterface.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(scale, nil)
	mockScaleInterface.EXPECT().Update(gomock.Any(), gomock.Any(), gomock.Eq(scale), gomock.Any())

	client.EXPECT().Status().Times(2).Return(statusWriter)
	statusWriter.EXPECT().Patch(gomock.Any(), gomock.Any(), gomock.Any()).Times(2)

	scaleExecutor.RequestScale(context.TODO(), &scaledObject, false, true)

	assert.Equal(t, int32(5), scale.Spec.Replicas)
	condition := scaledObject.Status.Conditions.GetFallbackCondition()
	assert.Equal(t, true, condition.IsTrue())
}
