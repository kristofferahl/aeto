mkdir aeto
cd aeto

git init
operator-sdk init --repo github.com/kristofferahl/aeto --domain aeto.net --project-name aeto
git add .
kubebuilder edit --multigroup=true
operator-sdk create api --group event --version v1alpha1 --kind EventStreamChunk --resource --controller
operator-sdk create api --group core --version v1alpha1 --kind Tenant --resource --controller
operator-sdk create api --group core --version v1alpha1 --kind ResourceTemplate --resource --controller
operator-sdk create api --group core --version v1alpha1 --kind Blueprint --resource --controller
operator-sdk create api --group core --version v1alpha1 --kind ResourceSet --resource --controller
operator-sdk create api --group route53.aws --version v1alpha1 --kind HostedZone --resource --controller
operator-sdk create api --group acm.aws --version v1alpha1 --kind Certificate --resource --controller
operator-sdk create api --group acm.aws --version v1alpha1 --kind CertificateConnector --resource --controller

github.com/aws/aws-sdk-go-v2/service/acm
github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2
github.com/aws/aws-sdk-go-v2/service/route53
