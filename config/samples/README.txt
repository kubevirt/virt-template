Create sample template by running:

kubectl create -f template_v1beta1_virtualmachinetemplate.yaml

Call subresources APIs by running:

kubectl create --raw /apis/subresources.template.kubevirt.io/v1beta1/namespaces/default/virtualmachinetemplates/virtualmachinetemplate-sample/process -f testparamsraw.json | jq .

or

kubectl create --raw /apis/subresources.template.kubevirt.io/v1beta1/namespaces/default/virtualmachinetemplates/virtualmachinetemplate-sample/create -f testparamsraw.json | jq .
