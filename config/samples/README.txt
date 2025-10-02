Create sample template by running:

kubectl create -f template_v1alpha1_virtualmachinetemplate.yaml

Call subresources APIs by running:

kubectl create --raw /apis/subresources.template.kubevirt.io/v1alpha1/namespaces/default/virtualmachinetemplates/virtualmachinetemplate-sample/process -f testparams.json | jq .

or

kubectl create --raw /apis/subresources.template.kubevirt.io/v1alpha1/namespaces/default/virtualmachinetemplates/virtualmachinetemplate-sample/create -f testparams.json | jq .
