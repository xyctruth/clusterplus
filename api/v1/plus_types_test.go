package v1

import (
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestValidPlus(t *testing.T) {
	tests := []struct {
		r     Plus
		isErr bool
	}{
		{r: Plus{ObjectMeta: metav1.ObjectMeta{Name: "aaa"}}, isErr: false},
		{r: Plus{ObjectMeta: metav1.ObjectMeta{Name: "aaa-aaa"}}, isErr: false},
		{r: Plus{ObjectMeta: metav1.ObjectMeta{Name: "aaa_aaa"}}, isErr: true},
		{r: Plus{ObjectMeta: metav1.ObjectMeta{Name: "aaa.aaa"}}, isErr: true},
		{r: Plus{ObjectMeta: metav1.ObjectMeta{Name: "aaa,aaa"}}, isErr: true},
	}
	for _, test := range tests {
		err := test.r.Validate()
		if test.isErr {
			require.NotNil(t, err)
		} else {
			require.Nil(t, err)
		}
	}
}
