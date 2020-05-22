package vocab

import (
	"bytes"
	"encoding/json"
	"goclassifieds/lib/entity"
	"log"

	"github.com/aws/aws-sdk-go/aws/session"
	elasticsearch7 "github.com/elastic/go-elasticsearch/v7"
)

type Vocabulary struct {
	Id          string `form:"id" json:"id"`
	UserId      string `form:"userId" json:"userId"`
	MachineName string `form:"machineName" json:"machineName" binding:"required"`
	HumanName   string `form:"humanName" json:"humanName" binding:"required"`
	Terms       []Term `form:"terms[]" json:"terms" binding:"required"`
}

type Term struct {
	Id           string `form:"id" json:"id" binding:"required"`
	VocabularyId string `form:"vocabularyId" json:"vocabularyId" binding:"required"`
	ParentId     string `form:"parentId" json:"parentId"`
	MachineName  string `form:"machineName" json:"machineName" binding:"required"`
	HumanName    string `form:"humanName" json:"humanName" binding:"required"`
	Weight       int    `form:"weight" json:"weight" binding:"required"`
	Group        bool   `form:"group" json:"group" binding:"required"`
	Selected     bool   `form:"selected" json:"selected" binding:"required"`
	Level        int    `form:"level" json:"level" binding:"required"`
	Children     []Term `form:"children" json:"children"`
}

func CreateVocabManager(esClient *elasticsearch7.Client, session *session.Session, userId string) entity.EntityManager {
	return entity.EntityManager{
		Config: entity.EntityConfig{
			SingularName: "vocabulary",
			PluralName:   "vocabularies",
			IdKey:        "id",
		},
		Loaders: map[string]entity.Loader{
			"s3": entity.S3LoaderAdaptor{
				Config: entity.S3AdaptorConfig{
					Session: session,
					Bucket:  "classifieds-ui-dev",
					Prefix:  "vocabularies/",
				},
			},
		},
		Storages: map[string]entity.Storage{
			"s3": entity.S3StorageAdaptor{
				Config: entity.S3AdaptorConfig{
					Session: session,
					Bucket:  "classifieds-ui-dev",
					Prefix:  "vocabularies/",
				},
			},
			"elastic": entity.ElasticStorageAdaptor{
				Config: entity.ElasticAdaptorConfig{
					Index:  "classified_vocabularies",
					Client: esClient,
				},
			},
		},
		Authorizers: map[string]entity.Authorization{
			"default": entity.OwnerAuthorizationAdaptor{
				Config: entity.OwnerAuthorizationConfig{
					UserId: userId,
				},
			},
		},
	}
}

func ToEntity(vocab *Vocabulary) (map[string]interface{}, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(vocab); err != nil {
		log.Fatalf("Error encoding query: %s", err)
	}
	jsonData, err := json.Marshal(vocab)
	if err != nil {
		return nil, err
	}
	var entity map[string]interface{}
	err = json.Unmarshal(jsonData, &entity)
	return entity, nil
}

func BuildVocabSearchQuery(userId string) map[string]interface{} {

	filterMust := []interface{}{
		map[string]interface{}{
			"term": map[string]interface{}{
				"userId.keyword": map[string]interface{}{
					"value": userId,
				},
			},
		},
	}

	query := map[string]interface{}{
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": []interface{}{
					map[string]interface{}{
						"bool": map[string]interface{}{
							"must": filterMust,
						},
					},
				},
			},
		},
	}

	return query
}
