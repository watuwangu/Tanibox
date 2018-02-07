package server

import (
	"net/http"
	"sort"
	"time"

	"github.com/Tanibox/tania-server/src/growth/domain"
	"github.com/Tanibox/tania-server/src/growth/query"
	"github.com/Tanibox/tania-server/src/growth/storage"
	"github.com/labstack/echo"
)

func (s *GrowthServer) SaveToCropReadModel(event interface{}) error {
	cropRead := &storage.CropRead{}

	switch e := event.(type) {
	case domain.CropBatchCreated:
		queryResult := <-s.AreaQuery.FindByID(e.InitialAreaUID)
		if queryResult.Error != nil {
			return queryResult.Error
		}

		srcArea, ok := queryResult.Result.(query.CropAreaQueryResult)
		if !ok {
			return echo.NewHTTPError(http.StatusInternalServerError, "Internal server error")
		}

		queryResult = <-s.MaterialQuery.FindByID(e.InventoryUID)
		if queryResult.Error != nil {
			return queryResult.Error
		}

		inv, ok := queryResult.Result.(query.CropMaterialQueryResult)
		if !ok {
			return echo.NewHTTPError(http.StatusInternalServerError, "Internal server error")
		}

		cropRead.UID = e.UID
		cropRead.BatchID = e.BatchID
		cropRead.Status = e.Status.Code
		cropRead.Type = e.Type.Code

		switch v := e.Container.Type.(type) {
		case domain.Tray:
			cropRead.Container = storage.Container{
				Type:     v.Code(),
				Cell:     v.Cell,
				Quantity: e.Quantity,
			}
		case domain.Pot:
			cropRead.Container = storage.Container{
				Type:     v.Code(),
				Quantity: e.Quantity,
			}
		}

		cropRead.Inventory = storage.Inventory{
			UID:       inv.UID,
			Name:      inv.Name,
			PlantType: inv.MaterialSeedPlantTypeCode,
		}

		cropRead.InitialArea = storage.InitialArea{
			AreaUID:         srcArea.UID,
			Name:            srcArea.Name,
			InitialQuantity: e.Quantity,
			CurrentQuantity: e.Quantity,
			CreatedDate:     e.CreatedDate,
			LastUpdated:     e.CreatedDate,
		}

		seeding := 0
		growing := 0
		if e.Type == domain.GetCropType(domain.CropTypeSeeding) {
			seeding += e.Quantity
		} else if e.Type == domain.GetCropType(domain.CropTypeGrowing) {
			growing += e.Quantity
		}

		cropRead.AreaStatus = storage.AreaStatus{
			Seeding: seeding,
			Growing: growing,
		}

		cropRead.FarmUID = e.FarmUID

	case domain.CropBatchMoved:
		queryResult := <-s.CropReadQuery.FindByID(e.UID)
		if queryResult.Error != nil {
			return queryResult.Error
		}

		cr, ok := queryResult.Result.(storage.CropRead)
		if !ok {
			return echo.NewHTTPError(http.StatusInternalServerError, "Internal server error")
		}

		cropRead = &cr

		queryResult = <-s.AreaQuery.FindByID(e.SrcAreaUID)
		if queryResult.Error != nil {
			return queryResult.Error
		}

		srcArea, ok := queryResult.Result.(query.CropAreaQueryResult)
		if !ok {
			return echo.NewHTTPError(http.StatusInternalServerError, "Internal server error")
		}

		queryResult = <-s.AreaQuery.FindByID(e.DstAreaUID)
		if queryResult.Error != nil {
			return queryResult.Error
		}

		dstArea, ok := queryResult.Result.(query.CropAreaQueryResult)
		if !ok {
			return echo.NewHTTPError(http.StatusInternalServerError, "Internal server error")
		}

		if cropRead.InitialArea.AreaUID == e.SrcAreaUID {
			ia, ok := e.UpdatedSrcArea.(domain.InitialArea)
			if ok {
				cropRead.InitialArea.CurrentQuantity = ia.CurrentQuantity
				cropRead.InitialArea.LastUpdated = ia.LastUpdated
			}
		}

		for i, v := range cropRead.MovedArea {
			ma, ok := e.UpdatedSrcArea.(domain.MovedArea)

			if ok {
				if v.AreaUID == ma.AreaUID {
					cropRead.MovedArea[i].CurrentQuantity = ma.CurrentQuantity
					cropRead.MovedArea[i].LastUpdated = ma.LastUpdated
				}
			}
		}

		if cropRead.InitialArea.AreaUID == e.DstAreaUID {
			ia, ok := e.UpdatedDstArea.(domain.InitialArea)
			if ok {
				cropRead.InitialArea.CurrentQuantity = ia.CurrentQuantity
				cropRead.InitialArea.LastUpdated = ia.LastUpdated
			}
		}

		isFound := false
		for i, v := range cropRead.MovedArea {
			ma, ok := e.UpdatedDstArea.(domain.MovedArea)
			if ok {
				if v.AreaUID == e.DstAreaUID {
					cropRead.MovedArea[i].CurrentQuantity = ma.CurrentQuantity
					cropRead.MovedArea[i].LastUpdated = ma.LastUpdated

					isFound = true
				}
			}
		}

		if !isFound {
			ma, ok := e.UpdatedDstArea.(domain.MovedArea)
			if ok {
				cropRead.MovedArea = append(cropRead.MovedArea, storage.MovedArea{
					AreaUID:         dstArea.UID,
					Name:            dstArea.Name,
					InitialQuantity: ma.InitialQuantity,
					CurrentQuantity: ma.CurrentQuantity,
					LastWatered:     &ma.LastWatered,
					LastPesticided:  &ma.LastPesticided,
					LastFertilized:  &ma.LastFertilized,
					LastPruned:      &ma.LastPruned,
					CreatedDate:     ma.CreatedDate,
					LastUpdated:     ma.LastUpdated,
				})
			}
		}

		if dstArea.Type == "SEEDING" {
			cropRead.AreaStatus.Seeding += e.Quantity
		}
		if dstArea.Type == "GROWING" {
			cropRead.AreaStatus.Growing += e.Quantity
		}
		if srcArea.Type == "SEEDING" {
			cropRead.AreaStatus.Seeding -= e.Quantity
		}
		if srcArea.Type == "GROWING" {
			cropRead.AreaStatus.Growing -= e.Quantity
		}

	case domain.CropBatchHarvested:
		queryResult := <-s.CropReadQuery.FindByID(e.UID)
		if queryResult.Error != nil {
			return queryResult.Error
		}

		cr, ok := queryResult.Result.(storage.CropRead)
		if !ok {
			return echo.NewHTTPError(http.StatusInternalServerError, "Internal server error")
		}

		cropRead = &cr

		queryResult = <-s.AreaQuery.FindByID(e.UpdatedHarvestedStorage.SourceAreaUID)
		if queryResult.Error != nil {
			return queryResult.Error
		}

		srcArea, ok := queryResult.Result.(query.CropAreaQueryResult)
		if !ok {
			return echo.NewHTTPError(http.StatusInternalServerError, "Internal server error")
		}

		hs := storage.HarvestedStorage{
			Quantity:             e.UpdatedHarvestedStorage.Quantity,
			ProducedGramQuantity: e.UpdatedHarvestedStorage.ProducedGramQuantity,
			SourceAreaUID:        srcArea.UID,
			SourceAreaName:       srcArea.Name,
			CreatedDate:          e.UpdatedHarvestedStorage.CreatedDate,
			LastUpdated:          e.UpdatedHarvestedStorage.LastUpdated,
		}

		isFound := false
		for i, v := range cropRead.HarvestedStorage {
			if v.SourceAreaUID == e.UpdatedHarvestedStorage.SourceAreaUID {
				cropRead.HarvestedStorage[i] = hs
				isFound = true
			}
		}

		if !isFound {
			cropRead.HarvestedStorage = append(cropRead.HarvestedStorage, hs)
		}

		if e.HarvestedAreaType == "INITIAL_AREA" {
			ha := e.HarvestedArea.(domain.InitialArea)
			cropRead.InitialArea.CurrentQuantity = ha.CurrentQuantity
			cropRead.InitialArea.LastUpdated = ha.LastUpdated
		} else if e.HarvestedAreaType == "MOVED_AREA" {
			ma := e.HarvestedArea.(domain.MovedArea)

			for i, v := range cropRead.MovedArea {
				if v.AreaUID == ma.AreaUID {
					cropRead.MovedArea[i].CurrentQuantity = ma.CurrentQuantity
					cropRead.MovedArea[i].LastUpdated = ma.LastUpdated
				}
			}
		}

		// Because Harvest should only be done in the GROWING area
		cropRead.AreaStatus.Growing -= e.HarvestedQuantity

	case domain.CropBatchDumped:
		queryResult := <-s.CropReadQuery.FindByID(e.UID)
		if queryResult.Error != nil {
			return queryResult.Error
		}

		cl, ok := queryResult.Result.(storage.CropRead)
		if !ok {
			return echo.NewHTTPError(http.StatusInternalServerError, "Internal server error")
		}

		cropRead = &cl

		queryResult = <-s.AreaQuery.FindByID(e.UpdatedTrash.SourceAreaUID)
		if queryResult.Error != nil {
			return queryResult.Error
		}

		srcArea, ok := queryResult.Result.(query.CropAreaQueryResult)
		if !ok {
			return echo.NewHTTPError(http.StatusInternalServerError, "Internal server error")
		}

		isFound := false
		for i, v := range cropRead.Trash {
			if v.SourceAreaUID == e.UpdatedTrash.SourceAreaUID {
				cropRead.Trash[i] = storage.Trash{
					Quantity:       e.UpdatedTrash.Quantity,
					SourceAreaUID:  srcArea.UID,
					SourceAreaName: srcArea.Name,
					LastUpdated:    e.DumpDate,
				}

				isFound = true
			}
		}

		if !isFound {
			cropRead.Trash = append(cropRead.Trash, storage.Trash{
				Quantity:       e.UpdatedTrash.Quantity,
				SourceAreaUID:  srcArea.UID,
				SourceAreaName: srcArea.Name,
				CreatedDate:    e.DumpDate,
				LastUpdated:    e.DumpDate,
			})
		}

		if e.DumpedAreaType == "INITIAL_AREA" {
			da := e.DumpedArea.(domain.InitialArea)
			cropRead.InitialArea.CurrentQuantity = da.CurrentQuantity
			cropRead.InitialArea.LastUpdated = da.LastUpdated

		} else if e.DumpedAreaType == "MOVED_AREA" {
			da := e.DumpedArea.(domain.MovedArea)

			for i, v := range cropRead.MovedArea {
				if v.AreaUID == da.AreaUID {
					cropRead.MovedArea[i].CurrentQuantity = da.CurrentQuantity
					cropRead.MovedArea[i].LastUpdated = da.LastUpdated
				}
			}
		}

		if srcArea.Type == "SEEDING" {
			cropRead.AreaStatus.Seeding -= e.Quantity
		}
		if srcArea.Type == "GROWING" {
			cropRead.AreaStatus.Growing -= e.Quantity
		}

	case domain.CropBatchWatered:
		queryResult := <-s.CropReadQuery.FindByID(e.UID)
		if queryResult.Error != nil {
			return queryResult.Error
		}

		cl, ok := queryResult.Result.(storage.CropRead)
		if !ok {
			return echo.NewHTTPError(http.StatusInternalServerError, "Internal server error")
		}

		cropRead = &cl

		if cropRead.InitialArea.AreaUID == e.AreaUID {
			cropRead.InitialArea.LastWatered = &e.WateringDate
		}

		for i, v := range cropRead.MovedArea {
			if v.AreaUID == e.AreaUID {
				cropRead.MovedArea[i].LastWatered = &e.WateringDate
			}
		}

	case domain.CropBatchNoteCreated:
		queryResult := <-s.CropReadQuery.FindByID(e.CropUID)
		if queryResult.Error != nil {
			return queryResult.Error
		}

		cr, ok := queryResult.Result.(storage.CropRead)
		if !ok {
			return echo.NewHTTPError(http.StatusInternalServerError, "Internal server error")
		}

		cropRead = &cr

		cropRead.Notes = append(cropRead.Notes, domain.CropNote{
			UID:         e.UID,
			Content:     e.Content,
			CreatedDate: e.CreatedDate,
		})

		sort.Slice(cropRead.Notes, func(i, j int) bool {
			return cropRead.Notes[i].CreatedDate.After(cropRead.Notes[j].CreatedDate)
		})

	case domain.CropBatchNoteRemoved:
		queryResult := <-s.CropReadQuery.FindByID(e.CropUID)
		if queryResult.Error != nil {
			return queryResult.Error
		}

		cr, ok := queryResult.Result.(storage.CropRead)
		if !ok {
			return echo.NewHTTPError(http.StatusInternalServerError, "Internal server error")
		}

		cropRead = &cr

		cropNoteTmp := []domain.CropNote{}
		for _, v := range cropRead.Notes {
			if v.UID != e.UID {
				cropNoteTmp = append(cropNoteTmp, v)
			}
		}

		cropRead.Notes = cropNoteTmp

	case domain.CropBatchPhotoCreated:
		queryResult := <-s.CropReadQuery.FindByID(e.CropUID)
		if queryResult.Error != nil {
			return queryResult.Error
		}

		cr, ok := queryResult.Result.(storage.CropRead)
		if !ok {
			return echo.NewHTTPError(http.StatusInternalServerError, "Internal server error")
		}

		cropRead = &cr

		cropRead.Photos = append(cropRead.Photos, storage.CropPhoto{
			UID:         e.UID,
			Filename:    e.Filename,
			MimeType:    e.MimeType,
			Size:        e.Size,
			Width:       e.Width,
			Height:      e.Height,
			Description: e.Description,
		})
	}

	err := <-s.CropReadRepo.Save(cropRead)
	if err != nil {
		return err
	}

	return nil
}

func (s *GrowthServer) SaveToCropActivityReadModel(event interface{}) error {
	cropActivity := &storage.CropActivity{}

	switch e := event.(type) {
	case domain.CropBatchCreated:
		queryResult := <-s.AreaQuery.FindByID(e.InitialAreaUID)
		if queryResult.Error != nil {
			return queryResult.Error
		}

		srcArea, ok := queryResult.Result.(query.CropAreaQueryResult)
		if !ok {
			return echo.NewHTTPError(http.StatusInternalServerError, "Internal server error")
		}

		cropActivity.UID = e.UID
		cropActivity.BatchID = e.BatchID
		cropActivity.ContainerType = e.Container.Type.Code()
		cropActivity.CreatedDate = time.Now()
		cropActivity.ActivityType = storage.SeedActivity{
			AreaUID:     srcArea.UID,
			AreaName:    srcArea.Name,
			Quantity:    e.Quantity,
			SeedingDate: e.CreatedDate,
		}

	case domain.CropBatchMoved:
		queryResult := <-s.CropReadQuery.FindByID(e.UID)
		if queryResult.Error != nil {
			return queryResult.Error
		}

		cr, ok := queryResult.Result.(storage.CropRead)
		if !ok {
			return echo.NewHTTPError(http.StatusInternalServerError, "Internal server error")
		}

		queryResult = <-s.AreaQuery.FindByID(e.SrcAreaUID)
		if queryResult.Error != nil {
			return queryResult.Error
		}

		srcArea, ok := queryResult.Result.(query.CropAreaQueryResult)
		if !ok {
			return echo.NewHTTPError(http.StatusInternalServerError, "Internal server error")
		}

		queryResult = <-s.AreaQuery.FindByID(e.DstAreaUID)
		if queryResult.Error != nil {
			return queryResult.Error
		}

		dstArea, ok := queryResult.Result.(query.CropAreaQueryResult)
		if !ok {
			return echo.NewHTTPError(http.StatusInternalServerError, "Internal server error")
		}

		cropActivity.UID = e.UID
		cropActivity.BatchID = cr.BatchID
		cropActivity.ContainerType = cr.Container.Type
		cropActivity.CreatedDate = time.Now()
		cropActivity.ActivityType = storage.MoveActivity{
			SrcAreaUID:  srcArea.UID,
			SrcAreaName: srcArea.Name,
			DstAreaUID:  dstArea.UID,
			DstAreaName: dstArea.Name,
			Quantity:    e.Quantity,
			MovedDate:   e.MovedDate,
		}

	case domain.CropBatchHarvested:
		queryResult := <-s.CropReadQuery.FindByID(e.UID)
		if queryResult.Error != nil {
			return queryResult.Error
		}

		cr, ok := queryResult.Result.(storage.CropRead)
		if !ok {
			return echo.NewHTTPError(http.StatusInternalServerError, "Internal server error")
		}

		queryResult = <-s.AreaQuery.FindByID(e.UpdatedHarvestedStorage.SourceAreaUID)
		if queryResult.Error != nil {
			return queryResult.Error
		}

		srcArea, ok := queryResult.Result.(query.CropAreaQueryResult)
		if !ok {
			return echo.NewHTTPError(http.StatusInternalServerError, "Internal server error")
		}

		cropActivity.UID = e.UID
		cropActivity.BatchID = cr.BatchID
		cropActivity.ContainerType = cr.Container.Type
		cropActivity.CreatedDate = time.Now()
		cropActivity.Description = e.Notes
		cropActivity.ActivityType = storage.HarvestActivity{
			SrcAreaUID:           srcArea.UID,
			SrcAreaName:          srcArea.Name,
			Quantity:             e.HarvestedQuantity,
			ProducedGramQuantity: e.ProducedGramQuantity,
			HarvestDate:          e.HarvestDate,
		}

	case domain.CropBatchDumped:
		queryResult := <-s.CropReadQuery.FindByID(e.UID)
		if queryResult.Error != nil {
			return queryResult.Error
		}

		cr, ok := queryResult.Result.(storage.CropRead)
		if !ok {
			return echo.NewHTTPError(http.StatusInternalServerError, "Internal server error")
		}

		queryResult = <-s.AreaQuery.FindByID(e.UpdatedTrash.SourceAreaUID)
		if queryResult.Error != nil {
			return queryResult.Error
		}

		srcArea, ok := queryResult.Result.(query.CropAreaQueryResult)
		if !ok {
			return echo.NewHTTPError(http.StatusInternalServerError, "Internal server error")
		}

		cropActivity.UID = e.UID
		cropActivity.BatchID = cr.BatchID
		cropActivity.ContainerType = cr.Container.Type
		cropActivity.CreatedDate = time.Now()
		cropActivity.Description = e.Notes
		cropActivity.ActivityType = storage.DumpActivity{
			SrcAreaUID:  srcArea.UID,
			SrcAreaName: srcArea.Name,
			Quantity:    e.Quantity,
			DumpDate:    e.DumpDate,
		}

	case domain.CropBatchWatered:
		cropActivity.UID = e.UID
		cropActivity.BatchID = e.BatchID
		cropActivity.ContainerType = e.ContainerType
		cropActivity.CreatedDate = time.Now()
		cropActivity.ActivityType = storage.WaterActivity{
			AreaUID:      e.AreaUID,
			AreaName:     e.AreaName,
			WateringDate: e.WateringDate,
		}

	case domain.CropBatchPhotoCreated:
		queryResult := <-s.CropReadQuery.FindByID(e.CropUID)
		if queryResult.Error != nil {
			return queryResult.Error
		}

		cr, ok := queryResult.Result.(storage.CropRead)
		if !ok {
			return echo.NewHTTPError(http.StatusInternalServerError, "Internal server error")
		}

		cropActivity.UID = e.CropUID
		cropActivity.BatchID = cr.BatchID
		cropActivity.ContainerType = cr.Container.Type
		cropActivity.CreatedDate = time.Now()
		cropActivity.ActivityType = storage.PhotoActivity{
			UID:         e.UID,
			Filename:    e.Filename,
			MimeType:    e.MimeType,
			Size:        e.Size,
			Width:       e.Width,
			Height:      e.Height,
			Description: e.Description,
		}
	}

	err := <-s.CropActivityRepo.Save(cropActivity)
	if err != nil {
		return err
	}

	return nil
}