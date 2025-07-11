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

var ErrExperimentNotFound = errors.New("experiment by id not found")

type ExperimentRepositoryImpl struct {
	*GenericRepository[models.Experiment, schema.Context, schema.ContextProperty, *models.ExperimentListOptions]
}

func NewExperimentRepository(db *gorm.DB, typeID int64) models.ExperimentRepository {
	config := GenericRepositoryConfig[models.Experiment, schema.Context, schema.ContextProperty, *models.ExperimentListOptions]{
		DB:                  db,
		TypeID:              typeID,
		EntityToSchema:      mapExperimentToContext,
		SchemaToEntity:      mapDataLayerToExperiment,
		EntityToProperties:  mapExperimentToContextProperties,
		NotFoundError:       ErrExperimentNotFound,
		EntityName:          "experiment",
		PropertyFieldName:   "context_id",
		ApplyListFilters:    applyExperimentListFilters,
		IsNewEntity:         func(entity models.Experiment) bool { return entity.GetID() == nil },
		HasCustomProperties: func(entity models.Experiment) bool { return entity.GetCustomProperties() != nil },
	}

	return &ExperimentRepositoryImpl{
		GenericRepository: NewGenericRepository(config),
	}
}

func (r *ExperimentRepositoryImpl) Save(experiment models.Experiment) (models.Experiment, error) {
	return r.GenericRepository.Save(experiment, nil)
}

func (r *ExperimentRepositoryImpl) List(listOptions models.ExperimentListOptions) (*models.ListWrapper[models.Experiment], error) {
	return r.GenericRepository.List(&listOptions)
}

func applyExperimentListFilters(query *gorm.DB, listOptions *models.ExperimentListOptions) *gorm.DB {
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
			queryBuilder := filter.NewQueryBuilderForRestEntity(filter.RestEntityExperiment)
			query = queryBuilder.BuildQuery(query, filterExpr)
		}
	}

	query = query.Scopes(scopes.Paginate(experiments, &listOptions.Pagination, r.db))

	if err := query.Find(&experimentsCtx).Error; err != nil {
		return nil, fmt.Errorf("error listing experiments: %w", err)
	}

	hasMore := false
	pageSize := listOptions.GetPageSize()
	if pageSize > 0 {
		hasMore = len(experimentsCtx) > int(pageSize)
		if hasMore {
			experimentsCtx = experimentsCtx[:len(experimentsCtx)-1]
		}
	}

	for _, expCtx := range experimentsCtx {
		propertiesCtx := []schema.ContextProperty{}
		if err := r.db.Where("context_id = ?", expCtx.ID).Find(&propertiesCtx).Error; err != nil {
			return nil, fmt.Errorf("error getting properties for experiment %d: %w", expCtx.ID, err)
		}
		experiment := mapDataLayerToExperiment(expCtx, propertiesCtx)
		experiments = append(experiments, experiment)
	}

	if hasMore && len(experimentsCtx) > 0 {
		lastExperiment := experimentsCtx[len(experimentsCtx)-1]
		orderBy := listOptions.GetOrderBy()
		value := ""
		if orderBy != "" {
			switch orderBy {
			case "ID":
				value = fmt.Sprintf("%d", lastExperiment.ID)
			case "CREATE_TIME":
				value = fmt.Sprintf("%d", lastExperiment.CreateTimeSinceEpoch)
			case "LAST_UPDATE_TIME":
				value = fmt.Sprintf("%d", lastExperiment.LastUpdateTimeSinceEpoch)
			default:
				value = fmt.Sprintf("%d", lastExperiment.ID)
			}
		}
		nextToken := scopes.CreateNextPageToken(lastExperiment.ID, value)
		listOptions.NextPageToken = &nextToken
	} else {
		listOptions.NextPageToken = nil
	}

	list.Items = experiments
	list.NextPageToken = listOptions.GetNextPageToken()
	list.PageSize = listOptions.GetPageSize()
	list.Size = int32(len(experiments))

	return &list, nil
}

func mapExperimentToContext(experiment models.Experiment) schema.Context {
	attrs := experiment.GetAttributes()
	context := schema.Context{
		TypeID: *experiment.GetTypeID(),
	}

	// Only set ID if it's not nil (for existing entities)
	if experiment.GetID() != nil {
		context.ID = *experiment.GetID()
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

func mapExperimentToContextProperties(experiment models.Experiment, contextID int32) []schema.ContextProperty {
	var properties []schema.ContextProperty

	if experiment.GetProperties() != nil {
		for _, prop := range *experiment.GetProperties() {
			properties = append(properties, MapPropertiesToContextProperty(prop, contextID, false))
		}
	}

	if experiment.GetCustomProperties() != nil {
		for _, prop := range *experiment.GetCustomProperties() {
			properties = append(properties, MapPropertiesToContextProperty(prop, contextID, true))
		}
	}

	return properties
}

func mapDataLayerToExperiment(expCtx schema.Context, propertiesCtx []schema.ContextProperty) models.Experiment {
	experimentModel := &models.BaseEntity[models.ExperimentAttributes]{
		ID:     &expCtx.ID,
		TypeID: &expCtx.TypeID,
		Attributes: &models.ExperimentAttributes{
			Name:                     &expCtx.Name,
			ExternalID:               expCtx.ExternalID,
			CreateTimeSinceEpoch:     &expCtx.CreateTimeSinceEpoch,
			LastUpdateTimeSinceEpoch: &expCtx.LastUpdateTimeSinceEpoch,
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
	experimentModel.Properties = &properties
	experimentModel.CustomProperties = &customProperties

	return experimentModel
}
