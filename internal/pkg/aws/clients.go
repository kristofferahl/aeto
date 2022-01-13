package aws

import (
	"context"
	"fmt"
	"strings"

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

// FindOneAcmCertificateByDomainName returns the first matching ACM Certificate by domain name
func (c Clients) FindOneAcmCertificateByDomainName(ctx context.Context, domainName string) (*acmtypes.CertificateSummary, error) {
	res, err := c.Acm.ListCertificates(ctx, &acm.ListCertificatesInput{})
	if err != nil {
		return nil, err
	}

	matches := make([]acmtypes.CertificateSummary, 0)

	for _, cert := range res.CertificateSummaryList {
		match := *cert.DomainName == domainName
		if match {
			matches = append(matches, cert)
		}
	}

	if len(matches) == 1 {
		return &matches[0], nil
	}

	if len(matches) > 0 {
		return nil, fmt.Errorf("multiple certificates found matching the domain name %s", domainName)
	}

	return nil, nil
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

// FindOneRoute53HostedZoneByName returns the first matching Route53 HostedZone by name
func (c Clients) FindOneRoute53HostedZoneByName(ctx context.Context, name string) (*route53types.HostedZone, error) {
	zones, err := c.Route53.ListHostedZonesByName(ctx, &route53.ListHostedZonesByNameInput{
		DNSName: aws.String(name),
	})
	if err != nil {
		return nil, err
	}

	matches := make([]route53types.HostedZone, 0)

	for _, zone := range zones.HostedZones {
		match := *zone.Name == name+"."
		if match {
			matches = append(matches, zone)
		}
	}

	if len(matches) == 1 {
		return &matches[0], nil
	}

	if len(matches) > 0 {
		return nil, fmt.Errorf("multiple hosted zones found matching the name %s", name)
	}

	return nil, nil
}

// SetRoute53HostedZoneTagsById adds, removes and updates tags for the Route53 HostedZone by Id
func (c Clients) SetRoute53HostedZoneTagsById(ctx context.Context, id string, tags map[string]string) error {
	id = strings.ReplaceAll(id, "/hostedzone/", "")

	tagsRes, err := c.Route53.ListTagsForResource(ctx, &route53.ListTagsForResourceInput{
		ResourceId:   aws.String(id),
		ResourceType: route53types.TagResourceTypeHostedzone,
	})
	if err != nil {
		return err
	}

	remove := make([]string, 0)
	add := make([]route53types.Tag, 0)

	for _, tag := range tagsRes.ResourceTagSet.Tags {
		if val, ok := tags[*tag.Key]; ok {
			if *tag.Value != val {
				remove = append(remove, *tag.Key)
			}
		} else {
			remove = append(remove, *tag.Key)
		}
	}

	for key, value := range tags {
		add = append(add, route53types.Tag{
			Key:   aws.String(key),
			Value: aws.String(value),
		})
	}

	params := route53.ChangeTagsForResourceInput{
		ResourceId:   aws.String(id),
		ResourceType: route53types.TagResourceTypeHostedzone,
	}

	if len(add) == 0 && len(remove) == 0 {
		return nil
	}

	if len(add) > 0 {
		params.AddTags = add
	}

	if len(remove) > 0 {
		params.RemoveTagKeys = remove
	}

	_, err = c.Route53.ChangeTagsForResource(ctx, &params)
	if err != nil {
		return err
	}

	return nil
}

// UpsertRoute53ResourceRecordSet creates or updates a resource recordset in the specified hosted zone
func (c Clients) UpsertRoute53ResourceRecordSet(ctx context.Context, hostedZoneId string, recordSet route53types.ResourceRecordSet, description string) error {
	return c.route53ResourceRecordSetAction(ctx, route53types.ChangeActionUpsert, hostedZoneId, recordSet, description)
}

// DeleteRoute53ResourceRecordSet deletes a resource recordset in the specified hosted zone
func (c Clients) DeleteRoute53ResourceRecordSet(ctx context.Context, hostedZoneId string, recordSet route53types.ResourceRecordSet, description string) error {
	return c.route53ResourceRecordSetAction(ctx, route53types.ChangeActionDelete, hostedZoneId, recordSet, description)
}

// DeleteRoute53ResourceRecordSet deletes a resource recordset in the specified hosted zone
func (c Clients) route53ResourceRecordSetAction(ctx context.Context, action route53types.ChangeAction, hostedZoneId string, recordSet route53types.ResourceRecordSet, description string) error {
	params := route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &route53types.ChangeBatch{
			Changes: []route53types.Change{
				{
					Action:            action,
					ResourceRecordSet: &recordSet,
				},
			},
			Comment: aws.String(description),
		},
		HostedZoneId: aws.String(hostedZoneId),
	}

	_, err := c.Route53.ChangeResourceRecordSets(ctx, &params)
	return err
}
