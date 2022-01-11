package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	route53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
)

// Clients wrapper for AWS
type Clients struct {
	Acm     *acm.Client
	Route53 *route53.Client
}

// GetAcmCertificateByDomainName returns the first matching ACM Certificate by domain name
func (c Clients) GetAcmCertificateByDomainName(ctx context.Context, domainName string) (acmtypes.CertificateSummary, error) {
	// TODO: If there are multiple matching certificates, return error

	res, err := c.Acm.ListCertificates(ctx, &acm.ListCertificatesInput{})
	if err != nil {
		return acmtypes.CertificateSummary{}, err
	}

	for _, cert := range res.CertificateSummaryList {
		match := *cert.DomainName == domainName
		if match {
			return cert, nil
		}
	}

	return acmtypes.CertificateSummary{}, fmt.Errorf("no AWS ACM Certificate found matching the domain name %s", domainName)
}

// GetAcmCertificateDetailsByArn returns ACM Certificate details by ARN
func (c Clients) GetAcmCertificateDetailsByArn(ctx context.Context, arn string) (acmtypes.CertificateDetail, error) {
	res, err := c.Acm.DescribeCertificate(ctx, &acm.DescribeCertificateInput{
		CertificateArn: aws.String(arn),
	})
	if err != nil {
		return acmtypes.CertificateDetail{}, err
	}

	return *res.Certificate, nil
}

// SetAcmCertificateTagsByArn adds, removes and updates tags for the ACM Certificate by ARN
func (c Clients) SetAcmCertificateTagsByArn(ctx context.Context, arn string, tags map[string]string) error {
	tagsRes, err := c.Acm.ListTagsForCertificate(ctx, &acm.ListTagsForCertificateInput{
		CertificateArn: aws.String(arn),
	})
	if err != nil {
		return err
	}

	remove := make([]acmtypes.Tag, 0)
	add := make([]acmtypes.Tag, 0)

	for _, tag := range tagsRes.Tags {
		if val, ok := tags[*tag.Key]; ok {
			if *tag.Value != val {
				remove = append(remove, tag)
			}
		} else {
			remove = append(remove, tag)
		}
	}

	for key, value := range tags {
		add = append(add, acmtypes.Tag{
			Key:   aws.String(key),
			Value: aws.String(value),
		})
	}

	if len(remove) > 0 {
		_, err := c.Acm.RemoveTagsFromCertificate(ctx, &acm.RemoveTagsFromCertificateInput{
			CertificateArn: aws.String(arn),
			Tags:           remove,
		})
		if err != nil {
			return err
		}
	}

	if len(add) > 0 {
		_, err := c.Acm.AddTagsToCertificate(ctx, &acm.AddTagsToCertificateInput{
			CertificateArn: aws.String(arn),
			Tags:           add,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// GetRoute53HostedZoneById returns Route53 HostedZone by ID
func (c Clients) GetRoute53HostedZoneById(ctx context.Context, id string) (route53types.HostedZone, error) {
	res, err := c.Route53.GetHostedZone(ctx, &route53.GetHostedZoneInput{
		Id: aws.String(id),
	})
	if err != nil {
		return route53types.HostedZone{}, err
	}

	return *res.HostedZone, nil
}

// GetRoute53HostedZoneByName returns the first matching Route53 HostedZone by name
func (c Clients) GetRoute53HostedZoneByName(ctx context.Context, name string) (route53types.HostedZone, error) {
	// TODO: If there are multiple matching hosted zones, return error

	zones, err := c.Route53.ListHostedZonesByName(ctx, &route53.ListHostedZonesByNameInput{
		DNSName: aws.String(name),
	})
	if err != nil {
		return route53types.HostedZone{}, err
	}

	for _, zone := range zones.HostedZones {
		isMatch := *zone.Name == name+"."
		if isMatch {
			return zone, nil
		}
	}

	return route53types.HostedZone{}, fmt.Errorf("no AWS Route53 HostedZone found matching the name %s", name)
}
