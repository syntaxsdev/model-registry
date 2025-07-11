package service

import (
	"errors"
	"fmt"

	"github.com/kubeflow/model-registry/internal/db/filter"
	"github.com/kubeflow/model-registry/internal/db/models"
	"github.com/kubeflow/model-registry/internal/db/schema"
	"github.com/kubeflow/model-registry/internal/db/scopes"
	"gorm.io/gorm"
)

var ErrRegisteredModelNotFound = errors.New("registered model by id not found")

type RegisteredModelRepositoryImpl struct {
	*GenericRepository[models.RegisteredModel, schema.Context, schema.ContextProperty, *models.RegisteredModelListOptions]
}

func NewRegisteredModelRepository(db *gorm.DB, typeID int64) models.RegisteredModelRepository {
	config := GenericRepositoryConfig[models.RegisteredModel, schema.Context, schema.ContextProperty, *models.RegisteredModelListOptions]{
		DB:                  db,
		TypeID:              typeID,
		EntityToSchema:      mapRegisteredModelToContext,
		SchemaToEntity:      mapDataLayerToRegisteredModel,
		EntityToProperties:  mapRegisteredModelToContextProperties,
		NotFoundError:       ErrRegisteredModelNotFound,
		EntityName:          "registered model",
		PropertyFieldName:   "context_id",
		ApplyListFilters:    applyRegisteredModelListFilters,
		IsNewEntity:         func(entity models.RegisteredModel) bool { return entity.GetID() == nil },
		HasCustomProperties: func(entity models.RegisteredModel) bool { return entity.GetCustomProperties() != nil },
	}

	return &RegisteredModelRepositoryImpl{
		GenericRepository: NewGenericRepository(config),
	}
}

func (r *RegisteredModelRepositoryImpl) Save(model models.RegisteredModel) (models.RegisteredModel, error) {
	return r.GenericRepository.Save(model, nil)
}

func (r *RegisteredModelRepositoryImpl) List(listOptions models.RegisteredModelListOptions) (*models.ListWrapper[models.RegisteredModel], error) {
	return r.GenericRepository.List(&listOptions)
}

func applyRegisteredModelListFilters(query *gorm.DB, listOptions *models.RegisteredModelListOptions) *gorm.DB {
	if listOptions.Name != nil {
		query = query.Where("name = ?", listOptions.Name)
	} else if listOptions.ExternalID != nil {
		query = query.Where("external_id = ?", listOptions.ExternalID)
	}

	// Apply filter query if provided
	if filterQuery := listOptions.GetFilterQuery(); filterQuery != "" {
		filterExpr, err := filter.Parse(filterQuery)
		if err != nil {
			return nil, fmt.Errorf("invalid filter query: %w", err)
		}

		if filterExpr != nil {
			queryBuilder := filter.NewQueryBuilderForRestEntity(filter.RestEntityRegisteredModel)
			query = queryBuilder.BuildQuery(query, filterExpr)
		}
	}

	query = query.Scopes(scopes.Paginate(models, &listOptions.Pagination, r.db))

	if err := query.Find(&modelsCtx).Error; err != nil {
		return nil, fmt.Errorf("error listing models: %w", err)
	}

	hasMore := false
	pageSize := listOptions.GetPageSize()
	if pageSize > 0 {
		hasMore = len(modelsCtx) > int(pageSize)
		if hasMore {
			modelsCtx = modelsCtx[:len(modelsCtx)-1]
		}
	}

	for _, modelCtx := range modelsCtx {
		propertiesCtx := []schema.ContextProperty{}
		if err := r.db.Where("context_id = ?", modelCtx.ID).Find(&propertiesCtx).Error; err != nil {
			return nil, fmt.Errorf("error getting properties for model %d: %w", modelCtx.ID, err)
		}
		model := mapDataLayerToRegisteredModel(modelCtx, propertiesCtx)
		models = append(models, model)
	}

	if hasMore && len(modelsCtx) > 0 {
		lastModel := modelsCtx[len(modelsCtx)-1]
		orderBy := listOptions.GetOrderBy()
		value := ""
		if orderBy != "" {
			switch orderBy {
			case "ID":
				value = fmt.Sprintf("%d", lastModel.ID)
			case "CREATE_TIME":
				value = fmt.Sprintf("%d", lastModel.CreateTimeSinceEpoch)
			case "LAST_UPDATE_TIME":
				value = fmt.Sprintf("%d", lastModel.LastUpdateTimeSinceEpoch)
			default:
				value = fmt.Sprintf("%d", lastModel.ID)
			}
		}
		nextToken := scopes.CreateNextPageToken(lastModel.ID, value)
		listOptions.NextPageToken = &nextToken
	} else {
		listOptions.NextPageToken = nil
	}

	list.Items = models
	list.NextPageToken = listOptions.GetNextPageToken()
	list.PageSize = listOptions.GetPageSize()
	list.Size = int32(len(models))

	return &list, nil
}

func mapRegisteredModelToContext(model models.RegisteredModel) schema.Context {
	attrs := model.GetAttributes()
	context := schema.Context{
		TypeID: *model.GetTypeID(),
	}

	// Only set ID if it's not nil (for existing entities)
	if model.GetID() != nil {
		context.ID = *model.GetID()
	}

	if attrs != nil {
		if attrs.Name != nil {
			context.Name = *attrs.Name
		}
		context.ExternalID = attrs.ExternalID
		if attrs.CreateTimeSinceEpoch != nil {
			context.CreateTimeSinceEpoch = *attrs.CreateTimeSinceEpoch
		}
		if attrs.LastUpdateTimeSinceEpoch != nil {
			context.LastUpdateTimeSinceEpoch = *attrs.LastUpdateTimeSinceEpoch
		}
	}

	return context
}

func mapRegisteredModelToContextProperties(model models.RegisteredModel, contextID int32) []schema.ContextProperty {
	var properties []schema.ContextProperty

	if model.GetProperties() != nil {
		for _, prop := range *model.GetProperties() {
			properties = append(properties, MapPropertiesToContextProperty(prop, contextID, false))
		}
	}

	if model.GetCustomProperties() != nil {
		for _, prop := range *model.GetCustomProperties() {
			properties = append(properties, MapPropertiesToContextProperty(prop, contextID, true))
		}
	}

	return properties
}

func mapDataLayerToRegisteredModel(modelCtx schema.Context, propertiesCtx []schema.ContextProperty) models.RegisteredModel {
	registeredModelModel := &models.BaseEntity[models.RegisteredModelAttributes]{
		ID:     &modelCtx.ID,
		TypeID: &modelCtx.TypeID,
		Attributes: &models.RegisteredModelAttributes{
			Name:                     &modelCtx.Name,
			ExternalID:               modelCtx.ExternalID,
			CreateTimeSinceEpoch:     &modelCtx.CreateTimeSinceEpoch,
			LastUpdateTimeSinceEpoch: &modelCtx.LastUpdateTimeSinceEpoch,
		},
	}

	properties := []models.Properties{}
	customProperties := []models.Properties{}

	for _, prop := range propertiesCtx {
		mappedProperty := MapContextPropertyToProperties(prop)

		if prop.IsCustomProperty {
			customProperties = append(customProperties, mappedProperty)
		} else {
			properties = append(properties, mappedProperty)
		}
	}

	// Always set Properties and CustomProperties, even if empty
	registeredModelModel.Properties = &properties
	registeredModelModel.CustomProperties = &customProperties

	return registeredModelModel
}
