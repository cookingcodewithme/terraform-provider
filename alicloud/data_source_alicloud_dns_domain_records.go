package alicloud

import (
	"fmt"
	"log"
	"regexp"

	"github.com/denverdino/aliyungo/dns"
	"github.com/hashicorp/terraform/helper/schema"
	"strings"
)

func dataSourceAlicloudDnsDomainRecords() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceAlicloudDnsDomainRecordsRead,

		Schema: map[string]*schema.Schema{
			"domain_name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"host_record_regex": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"value_regex": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"type": {
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				ValidateFunc: validateDomainRecordType,
			},
			"line": {
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				ValidateFunc: validateDomainRecordLine,
			},
			"status": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				ValidateFunc: func(v interface{}, k string) (ws []string, errors []error) {
					value := v.(string)
					if strings.ToLower(value) != "enable" && strings.ToLower(value) != "disable" {
						errors = append(errors, fmt.Errorf("%q must be 'enable' or 'distable', regardless of uppercase and lowercase.", k))
					}
					return
				},
			},
			"is_locked": {
				Type:     schema.TypeBool,
				Optional: true,
				ForceNew: true,
			},
			"output_file": {
				Type:     schema.TypeString,
				Optional: true,
			},

			// Computed values
			"records": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"record_id": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"domain_name": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"host_record": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"type": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"value": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"ttl": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"priority": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"line": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"status": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"locked": {
							Type:     schema.TypeBool,
							Computed: true,
						},
					},
				},
			},
		},
	}
}

func dataSourceAlicloudDnsDomainRecordsRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*AliyunClient).dnsconn

	args := &dns.DescribeDomainRecordsNewArgs{
		DomainName: d.Get("domain_name").(string),
	}
	if v, ok := d.GetOk("type"); ok && v.(string) != "" {
		args.TypeKeyWord = v.(string)
	}

	var allRecords []dns.RecordTypeNew

	pagination := getPagination(1, 50)
	for {
		args.Pagination = pagination
		resp, err := conn.DescribeDomainRecordsNew(args)
		if err != nil {
			return err
		}
		records := resp.DomainRecords.Record
		allRecords = append(allRecords, records...)

		if len(records) < pagination.PageSize {
			break
		}
		pagination.PageNumber += 1
	}

	var filteredRecords []dns.RecordTypeNew

	for _, record := range allRecords {
		if v, ok := d.GetOk("line"); ok && v.(string) != "" && strings.ToUpper(record.Line) != strings.ToUpper(v.(string)) {
			continue
		}

		if v, ok := d.GetOk("status"); ok && v.(string) != "" && strings.ToUpper(record.Status) != strings.ToUpper(v.(string)) {
			continue
		}

		if v, ok := d.GetOk("is_locked"); ok && record.Locked != v.(bool) {
			continue
		}

		if v, ok := d.GetOk("host_record_regex"); ok && v.(string) != "" {
			r := regexp.MustCompile(v.(string))
			if !r.MatchString(record.RR) {
				continue
			}
		}

		if v, ok := d.GetOk("value_regex"); ok && v.(string) != "" {
			r := regexp.MustCompile(v.(string))
			if !r.MatchString(record.Value) {
				continue
			}
		}

		filteredRecords = append(filteredRecords, record)
	}

	if len(filteredRecords) < 1 {
		return fmt.Errorf("Your query returned no results. Please change your search criteria and try again.")
	}
	log.Printf("[DEBUG] alicloud_dns_domain_records - Records found: %#v", allRecords)

	return recordsDecriptionAttributes(d, filteredRecords, meta)
}

func recordsDecriptionAttributes(d *schema.ResourceData, recordTypes []dns.RecordTypeNew, meta interface{}) error {
	var ids []string
	var s []map[string]interface{}
	for _, record := range recordTypes {
		mapping := map[string]interface{}{
			"record_id":   record.RecordId,
			"domain_name": record.DomainName,
			"line":        record.Line,
			"host_record": record.RR,
			"type":        record.Type,
			"value":       record.Value,
			"status":      record.Status,
			"locked":      record.Locked,
			"ttl":         record.TTL,
			"priority":    record.Priority,
		}
		log.Printf("[DEBUG] alicloud_dns_domain_records - adding record: %v", mapping)
		ids = append(ids, record.RecordId)
		s = append(s, mapping)
	}

	d.SetId(dataResourceIdHash(ids))
	if err := d.Set("records", s); err != nil {
		return err
	}

	// create a json file in current directory and write data source to it.
	if output, ok := d.GetOk("output_file"); ok && output != nil {
		writeToFile(output.(string), s)
	}
	return nil
}
