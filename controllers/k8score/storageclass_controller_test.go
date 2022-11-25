/*
Copyright ApeCloud Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package k8score

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"

	intctrlutil "github.com/apecloud/kubeblocks/internal/controllerutil"
)

var _ = Describe("Event Controller", func() {
	var (
		ctx      = context.Background()
		timeout  = time.Second * 20
		interval = time.Second
	)

	cleanupObjects := func() {
		err := k8sClient.DeleteAllOf(ctx, &storagev1.StorageClass{},
			client.InNamespace(testCtx.DefaultNamespace),
			client.HasLabels{testCtx.TestObjLabelKey})
		Expect(err).NotTo(HaveOccurred())
	}

	BeforeEach(func() {
		// Add any steup steps that needs to be executed before each test
		cleanupObjects()
	})

	AfterEach(func() {
		// Add any teardown steps that needs to be executed after each test
		cleanupObjects()
	})

	createStorageClassObj := func(storageClassName string, allowVolumeExpansion bool) *storagev1.StorageClass {
		By("By assure an default storageClass")
		scYAML := fmt.Sprintf(`
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: %s
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: hostpath.csi.k8s.io
reclaimPolicy: Delete
volumeBindingMode: Immediate
allowVolumeExpansion: %t
`, storageClassName, allowVolumeExpansion)
		sc := &storagev1.StorageClass{}
		Expect(yaml.Unmarshal([]byte(scYAML), sc)).Should(Succeed())
		Expect(testCtx.CreateObj(ctx, sc)).Should(Succeed())
		// wait until cluster created
		Eventually(func() bool {
			err := k8sClient.Get(context.Background(), client.ObjectKey{Name: storageClassName}, &storagev1.StorageClass{})
			return err == nil
		}, timeout, interval).Should(BeTrue())
		return sc
	}

	handleStorageClass := func(reqCtx intctrlutil.RequestCtx, cli client.Client, storageClass *storagev1.StorageClass) error {
		patch := client.MergeFrom(storageClass.DeepCopy())
		storageClass.Annotations["kubeblocks.io/test"] = "test"
		Expect(cli.Patch(ctx, storageClass, patch)).Should(Succeed())
		return nil
	}

	Context("When test creating storageClass", func() {
		It("should handle it properly", func() {
			By("test storageClass changes")
			StorageClassHandlerMap["test-controller"] = handleStorageClass
			storageClassName := fmt.Sprintf("standard-%s", testCtx.GetRandomStr())
			createStorageClassObj(storageClassName, true)
			storageClass := &storagev1.StorageClass{}
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: storageClassName}, storageClass)).Should(Succeed())
			allowVolumeExpansion := true
			storageClass.AllowVolumeExpansion = &allowVolumeExpansion
			Expect(k8sClient.Update(ctx, storageClass))

			// wait until storageClass patched
			Eventually(func() bool {
				tempStorageClass := &storagev1.StorageClass{}
				_ = k8sClient.Get(context.Background(), client.ObjectKey{Name: storageClass.Name}, tempStorageClass)
				return tempStorageClass.Annotations["kubeblocks.io/test"] == "test"
			}, timeout, interval).Should(BeTrue())
		})
	})
})
