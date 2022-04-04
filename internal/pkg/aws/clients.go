package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	route53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/go-logr/logr"
)

// Clients wrapper for AWS
type Clients struct {
	Log     logr.Logger
	Acm     *acm.Client
	Route53 *route53.Client
}

var unescaper = strings.NewReplacer(`\057`, "/", `\052`, "*")

type UniqueConstraintException struct {
	message string
}

func (e *UniqueConstraintException) Error() string {
	return "boom"
}

// FindOneAcmCertificateByDomainName returns the first matching ACM Certificate by domain name
func (c Clients) FindOneAcmCertificateByDomainName(ctx context.Context, domainName string) (*acmtypes.CertificateSummary, error) {
	items, err := c.ListAcmCertificates(ctx)
	if err != nil {
		return nil, err
	}

	matches := make([]acmtypes.CertificateSummary, 0)

	for _, cert := range items {
		match := *cert.DomainName == domainName
		if match {
			matches = append(matches, cert)
		}
	}

	if len(matches) == 1 {
		return &matches[0], nil
	}

	if len(matches) > 0 {
		return nil, &UniqueConstraintException{
			message: fmt.Sprintf("multiple certificates found matching the domain name %s", domainName),
		}
	}

	return nil, nil
}

// FindAcmCertificatesByDomainName returns the first matching ACM Certificate by domain name
func (c Clients) FindAcmCertificatesByDomainName(ctx context.Context, domainName string) ([]acmtypes.CertificateSummary, error) {
	items, err := c.ListAcmCertificates(ctx)
	if err != nil {
		return nil, err
	}

	matches := make([]acmtypes.CertificateSummary, 0)

	for _, cert := range items {
		match := *cert.DomainName == domainName
		if match {
			matches = append(matches, cert)
		}
	}

	return matches, nil
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

// ListAcmCertificates returns all ACM Certificates
func (c Clients) ListAcmCertificates(ctx context.Context) ([]acmtypes.CertificateSummary, error) {
	items := make([]acmtypes.CertificateSummary, 0)

	paginator := acm.NewListCertificatesPaginator(c.Acm, &acm.ListCertificatesInput{
		MaxItems: aws.Int32(5),
	})
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		items = append(items, output.CertificateSummaryList...)
	}

	return items, nil
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

// ListAcmCertificateTagsByArn lists tags for the ACM Certificate by ARN
func (c Clients) ListAcmCertificateTagsByArn(ctx context.Context, arn string) (tags map[string]string, err error) {
	tags = make(map[string]string)
	tagsRes, err := c.Acm.ListTagsForCertificate(ctx, &acm.ListTagsForCertificateInput{
		CertificateArn: aws.String(arn),
	})
	if err != nil {
		return tags, err
	}

	for _, tag := range tagsRes.Tags {
		tags[*tag.Key] = *tag.Value
	}

	return tags, nil
}

// FindOneRoute53HostedZoneByName returns the first matching Route53 HostedZone by name
func (c Clients) FindOneRoute53HostedZoneByName(ctx context.Context, name string) (*route53types.HostedZone, error) {
	items, err := c.ListRoute53HostedZones(ctx)
	if err != nil {
		return nil, err
	}

	matches := make([]route53types.HostedZone, 0)

	for _, zone := range items {
		match := *zone.Name == name+"."
		if match {
			matches = append(matches, zone)
		}
	}

	if len(matches) == 1 {
		return &matches[0], nil
	}

	if len(matches) > 0 {
		return nil, &UniqueConstraintException{
			message: fmt.Sprintf("multiple hosted zones found matching the name %s", name),
		}
	}

	return nil, nil
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

// DeleteRoute53HostedZone deletes a Route53 HostedZone by ID
func (c Clients) DeleteRoute53HostedZone(ctx context.Context, hostedZone route53types.HostedZone, force bool) error {
	if force {
		c.Log.V(1).Info("purging AWS Route53 HostedZone before deletion", "hosted-zone", hostedZone.Id)

		recordSetList, err := c.ListRoute53ResourceRecordSets(ctx, *hostedZone.Id)
		if err != nil {
			return err
		}

		changes := make([]route53types.Change, 0)
		for _, rs := range recordSetList {
			rs := rs
			if !isRoute53HostedZoneDefaultRecord(hostedZone, rs) {
				change := route53types.Change{
					Action:            route53types.ChangeActionDelete,
					ResourceRecordSet: &rs,
				}
				changes = append(changes, change)
			}
		}

		if len(changes) > 0 {
			params := route53.ChangeResourceRecordSetsInput{
				HostedZoneId: hostedZone.Id,
				ChangeBatch: &route53types.ChangeBatch{
					Changes: changes,
					Comment: aws.String("purging AWS Route53 HostedZone, cleaning up all resource record sets"),
				},
			}
			res, err := c.Route53.ChangeResourceRecordSets(ctx, &params)
			if err != nil {
				return err
			}
			c.WaitForRoute53Change(ctx, res.ChangeInfo)
			c.Log.Info(fmt.Sprintf("deleted %d resource record sets from AWS Route53 HostedZone", len(changes)), "hosted-zone", hostedZone.Id)
		}
	}

	res, err := c.Route53.DeleteHostedZone(ctx, &route53.DeleteHostedZoneInput{
		Id: hostedZone.Id,
	})
	if err != nil {
		return err
	}
	c.WaitForRoute53Change(ctx, res.ChangeInfo)

	return nil
}

// ListRoute53HostedZones returns all Route53 HostedZones
func (c Clients) ListRoute53HostedZones(ctx context.Context) ([]route53types.HostedZone, error) {
	items := make([]route53types.HostedZone, 0)

	paginator := route53.NewListHostedZonesPaginator(c.Route53, &route53.ListHostedZonesInput{})
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		items = append(items, output.HostedZones...)
	}

	return items, nil
}

// ListRoute53ResourceRecordSets returns all Route53 record sets for a HostedZone
func (c Clients) ListRoute53ResourceRecordSets(ctx context.Context, hostedZoneID string) ([]route53types.ResourceRecordSet, error) {
	items := make([]route53types.ResourceRecordSet, 0)

	req := route53.ListResourceRecordSetsInput{
		HostedZoneId: aws.String(hostedZoneID),
	}

	for {
		var resp *route53.ListResourceRecordSetsOutput
		resp, err := c.Route53.ListResourceRecordSets(ctx, &req)
		if err != nil {
			return items, err
		} else {
			items = append(items, resp.ResourceRecordSets...)
			if resp.IsTruncated {
				req.StartRecordName = resp.NextRecordName
				req.StartRecordType = resp.NextRecordType
				req.StartRecordIdentifier = resp.NextRecordIdentifier
			} else {
				break
			}
		}
	}

	// unescape wildcards
	for _, rrset := range items {
		rrset.Name = aws.String(unescaper.Replace(*rrset.Name))
	}

	return items, nil
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

func (c Clients) WaitForRoute53Change(ctx context.Context, change *route53types.ChangeInfo) {
	c.Log.V(1).Info("waiting for AWS Route53 change to complete", "id", change.Id)
	for {
		req := route53.GetChangeInput{Id: change.Id}
		resp, err := c.Route53.GetChange(ctx, &req)
		if err != nil {
			c.Log.V(1).Error(err, "waiting for AWS Route53 change failed", "id", change.Id)
		}
		if resp.ChangeInfo.Status == route53types.ChangeStatusInsync {
			c.Log.V(1).Info("AWS Route53 change completed", "id", change.Id)
			break
		} else if resp.ChangeInfo.Status == route53types.ChangeStatusPending {
			c.Log.V(2).Info("still wating for AWS Route53 change to complete", "id", change.Id)
		} else {
			c.Log.V(1).Info("AWS Route53 change failed", "id", change.Id, "status", resp.ChangeInfo.Status)
			break
		}
		time.Sleep(1 * time.Second)
	}
}

func isRoute53HostedZoneDefaultRecord(hostedZone route53types.HostedZone, recordSet route53types.ResourceRecordSet) bool {
	return (recordSet.Type == route53types.RRTypeNs || recordSet.Type == route53types.RRTypeSoa) && *recordSet.Name == *hostedZone.Name
}
