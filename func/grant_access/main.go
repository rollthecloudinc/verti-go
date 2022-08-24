package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"goclassifieds/lib/entity"
	"goclassifieds/lib/gov"
	"log"
	"os"
	"text/template"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/gocql/gocql"
	"github.com/tangzero/inflector"
)

type TemplateBindValueFunc func(value interface{}) string

type ResourceManagerParams struct {
	Session   *gocql.Session
	Request   *gov.GrantAccessRequest
	Resource  string
	Operation string
}

func handler(ctx context.Context, payload *gov.GrantAccessRequest) (gov.GrantAccessResponse, error) {

	cluster := gocql.NewCluster("cassandra.us-east-1.amazonaws.com")
	cluster.Keyspace = "ClassifiedsDev"
	cluster.Port = 9142
	cluster.Consistency = gocql.LocalOne // gocql.LocalQuorum
	cluster.Authenticator = &gocql.PasswordAuthenticator{Username: os.Getenv("KEYSPACE_USERNAME"), Password: os.Getenv("KEYSPACE_PASSWORD")}
	cluster.SslOpts = &gocql.SslOptions{Config: &tls.Config{ServerName: "cassandra.us-east-1.amazonaws.com"}, CaPath: "api/chat/AmazonRootCA1.pem", EnableHostVerification: true}
	cluster.PoolConfig = gocql.PoolConfig{HostSelectionPolicy: /*gocql.TokenAwareHostPolicy(*/ gocql.DCAwareRoundRobinPolicy("us-east-1") /*)*/}
	cSession, err := cluster.CreateSession()
	if err != nil {
		log.Fatal(err)
	}

	resourceParams := &ResourceManagerParams{
		Session:   cSession,
		Request:   payload,
		Resource:  fmt.Sprint(payload.Resource),
		Operation: fmt.Sprint(payload.Operation),
	}

	resourceManager, _ := ResourceManager(resourceParams)
	allAttributes := make([]entity.EntityAttribute, 0)
	data := &entity.EntityFinderDataBag{
		Attributes: allAttributes,
		Metadata: map[string]interface{}{
			"user":       payload.User,
			"type":       payload.Type,
			"resources":  gov.ResourceTypeMap,
			"operations": gov.OperationMap,
		},
	}
	results := resourceManager.Find("default", "default", data)

	/*b, _ := json.Marshal(results)
	log.Print(string(b))*/

	grant := len(results) != 0

	return gov.GrantAccessResponse{
		Grant: grant,
	}, nil

}

func ResourceManager(params *ResourceManagerParams) (entity.EntityManager, error) {
	entityName := "resource"
	bindings := &entity.VariableBindings{Values: make([]interface{}, 0)}
	funcMap := template.FuncMap{
		"bindValue": TemplateBindValue(bindings),
	}
	t, err := template.New("").Funcs(funcMap).Parse(Query())
	if err != nil {
		log.Printf("Error: %s", err.Error())
		return entity.EntityManager{}, err
	}
	manager := entity.EntityManager{
		Config: entity.EntityConfig{
			SingularName: entityName,
			PluralName:   inflector.Pluralize(entityName),
			IdKey:        "id",
			Stage:        os.Getenv("STAGE"),
		},
		Creator:  entity.DefaultCreatorAdaptor{},
		Storages: map[string]entity.Storage{},
		Finders: map[string]entity.Finder{
			"default": entity.CqlTemplateFinder{
				Config: entity.CqlTemplateFinderConfig{
					Session:  params.Session,
					Template: t,
					Table:    inflector.Pluralize(entityName),
					Bindings: bindings,
					Aliases:  map[string]string{},
				},
			},
		},
		Hooks: map[entity.Hooks]entity.EntityHook{},
		CollectionHooks: map[string]entity.EntityCollectionHook{
			"default/default": entity.PipeCollectionHooks(
				entity.FilterEntities(func(ent map[string]interface{}) bool {
					resource := fmt.Sprint(ent["resource"])
					op := fmt.Sprint(ent["op"])
					b, _ := json.Marshal(ent)
					log.Print(string(b))
					b2, _ := json.Marshal(params.Request)
					log.Print(string(b2))
					log.Print(resource == params.Resource)
					log.Print(ent["asset"] == params.Request.Asset)
					log.Print(op == params.Operation)
					return resource == params.Resource && ent["asset"] == params.Request.Asset && op == params.Operation
				}),
			),
		},
	}

	return manager, nil
}

func TemplateBindValue(bindings *entity.VariableBindings) TemplateBindValueFunc {
	return func(value interface{}) string {
		bindings.Values = append(bindings.Values, value)
		return "?"
	}
}

func Query() string {
	return `
	{{ define "default" }}
	SELECT
       resource,
			 asset,
		   op
	 FROM
			 resources
	WHERE 
			 user = {{ bindValue (index .Metadata "user" ) }}
		 AND
		   type = {{ bindValue (index .Metadata "type" ) }}
	{{end}}
	`
}

func main() {
	// Make the handler available for Remote Procedure Call by AWS Lambda
	lambda.Start(handler)
}