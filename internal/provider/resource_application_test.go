package provider

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"testing"
)

func Test_resourceApplication(t *testing.T) {
	randSuffix := acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)
	fullResourceName := "jumpcloud_application.example_app"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
		},
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			// Create step
			{
				Config: testApplicationConfig(randSuffix, "test_aws_account"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(fullResourceName, "display_label", "test_aws_account"),
				),
			},
			userImportStep(fullResourceName),
			// Update Step
			{
				Config: testApplicationConfig(randSuffix, "test_aws_account_updated"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(fullResourceName, "display_label", "test_aws_account_updated"),
				),
			},
			userImportStep(fullResourceName),
		},
	})
}

func Test_testIdPApplication(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
		},
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			// Create cognito
			{
				Config: fmt.Sprintf(`
resource "jumpcloud_application" "example_cognito_app" {
	name  				 = "amazoncognitouserpools"
	display_label        = "test_cognito_account"
	sso_url              = "https://sso.jumpcloud.com/saml2/amazoncognitouserpools"
   	idp_entity_id		 = "eu-central-1_UTFpIZF4F"
	sp_entity_id		 = "urn:amazon:cognito:sp:eu-central-1_UTFpIZF4F"
	acs_url				 = "https://sagwave.auth.eu-central-1.amazoncognito.com"
}
`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("jumpcloud_application.example_cognito_app", "display_label", "test_cognito_account"),
					resource.TestCheckResourceAttr("jumpcloud_application.example_cognito_app", "idp_entity_id", "eu-central-1_UTFpIZF4F"),
				),
			},
			userImportStep("jumpcloud_application.example_cognito_app"),
		},
	})
}

func testApplicationConfig(randSuffix string, displayLabel string) string {
	return fmt.Sprintf(`
resource "jumpcloud_application" "example_app" {
	name  				 = "aws"
	display_label        = "%s"
	sso_url              = "https://sso.jumpcloud.com/saml2/example-application_%s"
   constant_attribute {
       name = "https://aws.amazon.com/SAML/Attributes/Role"
       value = "arn:aws:iam::AWS_ACCOUNT_ID:role/MY_ROLE,arn:aws:iam::AWS_ACCOUNT_ID:saml-provider/MY_SAML_PROVIDER"
   }
   constant_attribute {
       name = "https://aws.amazon.com/SAML/Attributes/SessionDuration"
       value = 43200
   }
}
`, displayLabel, randSuffix)
}
