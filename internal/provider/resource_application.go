package provider

import (
	"fmt"
	jcapiv1 "github.com/TheJumpCloud/jcapi-go/v1"
	jcapiv2 "github.com/TheJumpCloud/jcapi-go/v2"
	"github.com/hashicorp/terraform-plugin-sdk/helper/hashcode"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/structure"
	"golang.org/x/net/context"
	"log"
)

const (
	AttributeNameAwsSessionDuration = "https://aws.amazon.com/SAML/Attributes/SessionDuration"
	AttributeNameAwsRole            = "https://aws.amazon.com/SAML/Attributes/Role"
)

func resourceConstantAttributeSetting() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"value": {
				Type:     schema.TypeString,
				Required: true,
			},
		},
	}
}

func resourceApplication() *schema.Resource {
	return &schema.Resource{
		Description:   "Provides a resource for adding an Amazon Web Services (AWS) account application. **Note:** This resource is due to change in future versions to be more generic and allow for adding various applications supported by JumpCloud.",
		CreateContext: resourceApplicationCreate,
		ReadContext:   resourceApplicationRead,
		UpdateContext: resourceApplicationUpdate,
		DeleteContext: resourceApplicationDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Description: "Name of the application",
				Type:        schema.TypeString,
				Required:    true,
			},
			"display_label": {
				Description: "Name of the application to display",
				Type:        schema.TypeString,
				Required:    true,
			},
			"sso_url": {
				Description: "The SSO URL suffix to use",
				Type:        schema.TypeString,
				Required:    true,
			},
			"constant_attribute": {
				Description: "TODO",
				Optional:    true,
				Type:        schema.TypeSet,
				Elem:        resourceConstantAttributeSetting(),
				Set:         optionConstantAttributeValueHash,
			},
			"metadata_xml": {
				Description: "The JumpCloud metadata XML file.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"idp_entity_id": {
				Description: "Also referred to as the 'Issuer', this is the unique, case-sensitive identifier used by JumpCloud for this service provider",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"sp_entity_id": {
				Description: "Also referred to as the 'Audience', this is the unique, case-sensitive identifier by this service provider.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"acs_url": {
				Description: "Assertion Consumer Service URL, also referred to as the 'Recipient'.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			// TODO get idpInitUrl once the SDK supports it

		},
	}
}

func optionConstantAttributeValueHash(v interface{}) int {
	rd := v.(map[string]interface{})
	name := rd["name"].(string)
	value := rd["value"].(string)
	value, _ = structure.NormalizeJsonString(value)
	hk := fmt.Sprintf("%s:%s", name, value)
	return hashcode.String(hk)
}

func resourceApplicationCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	configv1 := convertV2toV1Config(meta.(*jcapiv2.Configuration))
	client := jcapiv1.NewAPIClient(configv1)

	payload := generateAwsPayload(d)
	request := map[string]interface{}{
		"body": payload,
	}

	returnStruct, _, err := client.ApplicationsApi.ApplicationsPost(context.TODO(), request)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(returnStruct.Id)
	return resourceApplicationRead(ctx, d, meta)
}

func resourceApplicationRead(_ context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	configv1 := convertV2toV1Config(meta.(*jcapiv2.Configuration))
	client := jcapiv1.NewAPIClient(configv1)

	response, _, err := client.ApplicationsApi.ApplicationsGet(context.TODO(), d.Id(), nil)

	// If the object does not exist, unset the ID
	if err != nil {
		if err.Error() == "EOF" {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	d.SetId(response.Id)

	if err := d.Set("name", response.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("display_label", response.DisplayLabel); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("sso_url", response.SsoUrl); err != nil {
		return diag.FromErr(err)
	}
	if _, ok := d.GetOk("idp_entity_id"); ok {
		if err := d.Set("idp_entity_id", response.Config.IdpEntityId.Value); err != nil {
			return diag.FromErr(err)
		}
	}
	if _, ok := d.GetOk("sp_entity_id"); ok {
		if err := d.Set("sp_entity_id", response.Config.SpEntityId.Value); err != nil {
			return diag.FromErr(err)
		}
	}
	if _, ok := d.GetOk("acs_url"); ok {
		if err := d.Set("acs_url", response.Config.AcsUrl.Value); err != nil {
			return diag.FromErr(err)
		}
	}

	if _, ok := d.GetOk("constant_attribute"); ok {
		allConstantAttributeValues := response.Config.ConstantAttributes.Value
		var elements []interface{}
		for _, el := range allConstantAttributeValues {
			v := map[string]interface{}{
				"name":  el.Name,
				"value": el.Value,
			}
			elements = append(elements, v)
		}
		updatedConstantAttributes := schema.NewSet(optionConstantAttributeValueHash, elements)
		if err := d.Set("constant_attribute", updatedConstantAttributes.List()); err != nil {
			return diag.FromErr(err)
		}
	}

	if response.Id != "" {
		log.Println("[INFO] response ID is ", response.Id)
		orgId := configv1.DefaultHeader["x-org-id"]
		apiKey := configv1.DefaultHeader["x-api-key"]

		metadataXml, err := GetApplicationMetadataXml(orgId, response.Id, apiKey)
		if err != nil {
			return diag.FromErr(err)
		}

		if err := d.Set("metadata_xml", metadataXml); err != nil {
			return diag.FromErr(err)
		}
	} else {
		log.Println("[INFO] no ID in response, skipping metadata XML retrieval")
	}

	return nil
}

func resourceApplicationUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	configv1 := convertV2toV1Config(meta.(*jcapiv2.Configuration))
	client := jcapiv1.NewAPIClient(configv1)

	payload := generateAwsPayload(d)
	request := map[string]interface{}{
		"body": payload,
	}

	_, _, err := client.ApplicationsApi.ApplicationsPut(context.TODO(), d.Id(), request)
	if err != nil {
		return diag.FromErr(err)
	}
	return resourceApplicationRead(ctx, d, meta)
}

func resourceApplicationDelete(_ context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	configv1 := convertV2toV1Config(meta.(*jcapiv2.Configuration))
	client := jcapiv1.NewAPIClient(configv1)

	_, _, err := client.ApplicationsApi.ApplicationsDelete(context.TODO(), d.Id(), nil)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}

func generateAwsPayload(d *schema.ResourceData) jcapiv1.Application {
	if v, ok := d.GetOk("constant_attribute"); ok {
		constantAttributes := v.(*schema.Set)
		payload := []jcapiv1.ApplicationConfigConstantAttributesValue{}

		for _, constantAttribute := range constantAttributes.List() {
			v := jcapiv1.ApplicationConfigConstantAttributesValue{
				Name:  constantAttribute.(map[string]interface{})["name"].(string),
				Value: constantAttribute.(map[string]interface{})["value"].(string),
			}
			payload = append(payload, v)
		}
		return jcapiv1.Application{
			Beta:         false,
			Name:         d.Get("name").(string),
			DisplayLabel: d.Get("display_label").(string),
			SsoUrl:       d.Get("sso_url").(string),
			Config: &jcapiv1.ApplicationConfig{
				ConstantAttributes: &jcapiv1.ApplicationConfigConstantAttributes{
					Value: payload,
				},
			},
		}
	} else {
		return jcapiv1.Application{
			Beta:         true,
			Name:         d.Get("name").(string),
			DisplayLabel: d.Get("display_label").(string),
			SsoUrl:       d.Get("sso_url").(string),
			Config: &jcapiv1.ApplicationConfig{
				IdpEntityId: &jcapiv1.ApplicationConfigAcsUrl{Value: d.Get("idp_entity_id").(string)},
				SpEntityId:  &jcapiv1.ApplicationConfigAcsUrl{Value: d.Get("sp_entity_id").(string)},
				AcsUrl:      &jcapiv1.ApplicationConfigAcsUrl{Value: d.Get("acs_url").(string)},
				// TODO set idpInitUrl once the SDK supports it
			},
		}
	}

}
