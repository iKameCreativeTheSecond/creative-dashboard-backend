package db_handler

import (
	"context"
	"log"
	"os"
	collectionmodels "performance-dashboard-backend/internal/database/collection_models"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var client *mongo.Client

func ConnectMongoDB() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	mongoURI := os.Getenv("MONGO_URI")
	var err error
	client, err = mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		return err
	}
	// Optionally, ping the database to verify connection
	if err := client.Ping(ctx, nil); err != nil {
		return err
	}
	log.Println("Connected to MongoDB!")
	return nil
}

type Team struct {
	ID string `bson:"_id"`
}

type TeamWeeklyTarget struct {
	Team         string `bson:"team"`
	WeeklyTarget int32  `bson:"point"`
}

type PerformancePoint struct {
	StartWeek                 time.Time `bson:"_id"`
	TotalPerformancePoint     float64   `bson:"total_performance_point"`
	TotalCreativeProcessPoint float64   `bson:"total_creative_process_point"`
	TotalCreativeTaskPoint    float64   `bson:"total_creative_task_point"`
	TotalBasePoint            float64   `bson:"total_base_point"`
	Identifier                string    `bson:"identifier"`
}

type TeamRole struct {
	Team string `bson:"team"`
	Role string `bson:"role"`
}

func GetPerformancePoint(uri, dbName, collName, identifier string, startDate, endDate time.Time, isTeam bool) ([]*PerformancePoint, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// defer client.Disconnect(ctx)
	collection := client.Database(dbName).Collection(collName)

	var identifierKey string
	if isTeam {
		identifierKey = "team"
	} else {
		identifierKey = "assignee_id"
	}

	pipeline := mongo.Pipeline{
		// 1. $match
		{{Key: "$match", Value: bson.D{
			{Key: identifierKey, Value: identifier},
			{Key: "done_date", Value: bson.D{
				{Key: "$gte", Value: startDate},
				{Key: "$lte", Value: endDate},
			}},
		}}},
		// 2. $addFields: week_start via $dateTrunc
		{{Key: "$addFields", Value: bson.D{
			{Key: "week_start", Value: bson.D{
				{Key: "$dateTrunc", Value: bson.D{
					{Key: "date", Value: "$done_date"},
					{Key: "unit", Value: "week"},
					{Key: "binSize", Value: 1},
					{Key: "startOfWeek", Value: "monday"},
				}},
			}},
		}}},
		// 3. $lookup level theo team
		{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: "level"},
			{Key: "localField", Value: "team"},
			{Key: "foreignField", Value: "team"},
			{Key: "as", Value: "level_info"},
		}}},
		{{Key: "$unwind", Value: "$level_info"}},

		// 4. Tính performance_point từ levelPoint
		{{Key: "$addFields", Value: bson.D{
			{Key: "performance_point", Value: bson.D{
				{Key: "$let", Value: bson.D{
					{Key: "vars", Value: bson.D{
						{Key: "idx", Value: bson.D{{Key: "$subtract", Value: bson.A{"$level", 1}}}},
						{Key: "arr", Value: "$level_info.levelPoint"},
					}},
					{Key: "in", Value: bson.D{
						{Key: "$cond", Value: bson.A{
							bson.D{{Key: "$lt", Value: bson.A{"$$idx", bson.D{{Key: "$size", Value: "$$arr"}}}}},
							bson.D{{Key: "$arrayElemAt", Value: bson.A{"$$arr", "$$idx"}}},
							"$level",
						}},
					}},
				}},
			}},
		}}},

		// 5. $lookup creative-tool theo tool
		{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: "creative-tool"},
			{Key: "localField", Value: "tool"},
			{Key: "foreignField", Value: "index"},
			{Key: "as", Value: "tool_info"},
		}}},

		// 6. Tách tool loại t và q, map points
		{{Key: "$addFields", Value: bson.D{
			{Key: "tool_points_t", Value: bson.D{
				{Key: "$map", Value: bson.D{
					{Key: "input", Value: bson.D{
						{Key: "$filter", Value: bson.D{
							{Key: "input", Value: "$tool_info"},
							{Key: "as", Value: "t"},
							{Key: "cond", Value: bson.D{{Key: "$eq", Value: bson.A{"$$t.type", "t"}}}},
						}},
					}},
					{Key: "as", Value: "t"},
					{Key: "in", Value: bson.D{
						{Key: "index", Value: "$$t.index"},
						{Key: "point", Value: bson.D{
							{Key: "$let", Value: bson.D{
								{Key: "vars", Value: bson.D{
									{Key: "idx", Value: bson.D{{Key: "$subtract", Value: bson.A{"$level", 1}}}},
									{Key: "arr", Value: "$$t.point"},
								}},
								{Key: "in", Value: bson.D{
									{Key: "$cond", Value: bson.A{
										bson.D{{Key: "$lt", Value: bson.A{"$$idx", bson.D{{Key: "$size", Value: "$$arr"}}}}},
										bson.D{{Key: "$arrayElemAt", Value: bson.A{"$$arr", "$$idx"}}},
										bson.D{{Key: "$arrayElemAt", Value: bson.A{"$$arr", 0}}},
									}},
								}},
							}},
						}},
					}},
				}},
			}},
			{Key: "tool_points_q", Value: bson.D{
				{Key: "$map", Value: bson.D{
					{Key: "input", Value: bson.D{
						{Key: "$filter", Value: bson.D{
							{Key: "input", Value: "$tool_info"},
							{Key: "as", Value: "t"},
							{Key: "cond", Value: bson.D{{Key: "$eq", Value: bson.A{"$$t.type", "q"}}}},
						}},
					}},
					{Key: "as", Value: "t"},
					{Key: "in", Value: bson.D{
						{Key: "index", Value: "$$t.index"},
						{Key: "point", Value: bson.D{{Key: "$arrayElemAt", Value: bson.A{"$$t.point", 0}}}},
					}},
				}},
			}},
		}}},

		// 7. Tính tool_factor và creative_process_point
		{{Key: "$addFields", Value: bson.D{
			{Key: "tool_factor", Value: bson.D{
				{Key: "$cond", Value: bson.D{
					{Key: "if", Value: bson.D{{Key: "$eq", Value: bson.A{bson.D{{Key: "$size", Value: "$tool_points_t"}}, 0}}}},
					{Key: "then", Value: 0},
					{Key: "else", Value: bson.D{
						{Key: "$reduce", Value: bson.D{
							{Key: "input", Value: bson.D{
								{Key: "$map", Value: bson.D{
									{Key: "input", Value: "$tool_points_t"},
									{Key: "as", Value: "tp"},
									{Key: "in", Value: bson.D{
										{Key: "$cond", Value: bson.A{
											bson.D{{Key: "$lt", Value: bson.A{"$$tp.point", 1}}},
											"$$tp.point",
											1,
										}},
									}},
								}},
							}},
							{Key: "initialValue", Value: 1},
							{Key: "in", Value: bson.D{{Key: "$multiply", Value: bson.A{"$$value", "$$this"}}}},
						}},
					}},
				}},
			}},
			{Key: "creative_process_point", Value: bson.D{
				{Key: "$sum", Value: bson.D{
					{Key: "$map", Value: bson.D{
						{Key: "input", Value: "$tool_points_q"},
						{Key: "as", Value: "tp"},
						{Key: "in", Value: "$$tp.point"},
					}},
				}},
			}},
		}}},

		// 8. creative_task_point = performance_point * tool_factor
		{{Key: "$addFields", Value: bson.D{
			{Key: "creative_task_point", Value: bson.D{
				{Key: "$multiply", Value: bson.A{"$performance_point", "$tool_factor"}},
			}},
		}}},

		// 9. base_point
		{{Key: "$addFields", Value: bson.D{
			{Key: "base_point", Value: bson.D{
				{Key: "$subtract", Value: bson.A{"$performance_point", "$creative_task_point"}},
			}},
		}}},

		// 10. $project các field cần thiết
		{{Key: "$project", Value: bson.D{
			{Key: "task_name", Value: 1},
			{Key: "assignee_id", Value: 1},
			{Key: "team", Value: 1},
			{Key: "level", Value: 1},
			{Key: "performance_point", Value: 1},
			{Key: "tool_points_t", Value: 1},
			{Key: "tool_points_q", Value: 1},
			{Key: "tool_factor", Value: 1},
			{Key: "creative_process_point", Value: 1},
			{Key: "creative_task_point", Value: 1},
			{Key: "base_point", Value: 1},
			{Key: "done_date", Value: 1},
			{Key: "week_start", Value: 1},
		}}},

		// 11. Group theo tuần
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$week_start"},
			{Key: "total_performance_point", Value: bson.D{{Key: "$sum", Value: "$performance_point"}}},
			{Key: "total_creative_process_point", Value: bson.D{{Key: "$sum", Value: "$creative_process_point"}}},
			{Key: "total_creative_task_point", Value: bson.D{{Key: "$sum", Value: "$creative_task_point"}}},
			{Key: "total_base_point", Value: bson.D{{Key: "$sum", Value: "$base_point"}}},
			{Key: "tasks", Value: bson.D{{Key: "$push", Value: "$$ROOT"}}},
		}}},

		// 12. Sort theo tuần
		{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []*PerformancePoint
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, nil
	}

	for _, r := range results {
		r.Identifier = identifier
	}
	return results, nil
}

func GetMembersByTeam(uri, dbName, collName string, team string) ([]*collectionmodels.Member, error) {
	// Example body request
	// 	{
	//     "teams": ["Art Creative"]
	// 	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	// defer client.Disconnect(ctx)
	collection := client.Database(dbName).Collection(collName)

	// if team == "" or null return all collection
	if team == "" {
		cursor, err := collection.Find(ctx, bson.M{})
		if err != nil {
			return nil, err
		}
		defer cursor.Close(ctx)

		var results []*collectionmodels.Member
		if err = cursor.All(ctx, &results); err != nil {
			return nil, err
		}
		return results, nil
	}

	filter := bson.M{
		"team": team,
	}
	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []*collectionmodels.Member
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, nil
	}

	return results, nil
}

func IsEmailInDatabase(uri, dbName, collName, email string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collName)
	filter := bson.M{"email": email}
	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func GetMemberRoles(uri, dbName, collName, email string) ([]*TeamRole, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collName)

	pipeline := mongo.Pipeline{
		{{
			Key: "$match", Value: bson.D{{Key: "email", Value: email}},
		}},
		{{
			Key: "$project", Value: bson.D{
				{Key: "role", Value: 1},
				{Key: "team", Value: 1},
				{Key: "_id", Value: 0},
			},
		}},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []*TeamRole
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func GetAllTeams(uri, dbName, collName string) ([]*Team, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collName)
	pipeline := mongo.Pipeline{
		{{
			Key: "$group", Value: bson.D{
				{Key: "_id", Value: "$team"},
			},
		}},
	}
	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []*Team
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func GetTeamWeeklyTarget(uri, dbName, collName, team string) (*TeamWeeklyTarget, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collName)
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.D{
			{Key: "date_from", Value: bson.D{{Key: "$lte", Value: time.Now()}}},
			{Key: "date_to", Value: bson.D{{Key: "$gte", Value: time.Now()}}},
			{Key: "team", Value: team},
		}}},
		bson.D{{Key: "$project", Value: bson.D{
			{Key: "_id", Value: 0},
			{Key: "team", Value: 1},
			{Key: "point", Value: 1},
		}}},
	}

	var result TeamWeeklyTarget
	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if cursor.Next(ctx) {
		if err := cursor.Decode(&result); err != nil {
			return nil, err
		}
		return &result, nil
	}
	return nil, nil
}

func GetMongoClient() *mongo.Client {
	return client
}

// Lấy tổng điểm trong khoảng thời gian, không chia theo tuần
type PerformancePointTotal struct {
	TotalPerformancePoint     float64 `bson:"total_performance_point"`
	TotalCreativeProcessPoint float64 `bson:"total_creative_process_point"`
	TotalCreativeTaskPoint    float64 `bson:"total_creative_task_point"`
	TotalBasePoint            float64 `bson:"total_base_point"`
	Identifier                string  `bson:"identifier"`
}

type PerformancePointTotalWithTime struct {
	StartDate             time.Time             `bson:"start_date"`
	EndDate               time.Time             `bson:"end_date"`
	TotalPerformancePoint PerformancePointTotal `bson:"total_performance_point"`
}

func GetPerformancePointTotal(uri, dbName, collName, identifier string, startDate, endDate time.Time, isTeam bool) (*PerformancePointTotal, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collName)

	var identifierKey string
	if isTeam {
		identifierKey = "team"
	} else {
		identifierKey = "assignee_id"
	}

	pipeline := mongo.Pipeline{
		// 1. Match theo done_date
		{{Key: "$match", Value: bson.D{
			{Key: identifierKey, Value: identifier},
			{Key: "done_date", Value: bson.D{
				{Key: "$gte", Value: startDate},
				{Key: "$lte", Value: endDate},
			}},
		}}},

		// 2. Lookup level theo team
		{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: "level"},
			{Key: "localField", Value: "team"},
			{Key: "foreignField", Value: "team"},
			{Key: "as", Value: "level_info"},
		}}},
		{{Key: "$unwind", Value: "$level_info"}},

		// 3. Tính performance_point
		{{Key: "$addFields", Value: bson.D{
			{Key: "performance_point", Value: bson.D{
				{Key: "$let", Value: bson.D{
					{Key: "vars", Value: bson.D{
						{Key: "idx", Value: bson.D{{Key: "$subtract", Value: bson.A{"$level", 1}}}},
						{Key: "arr", Value: "$level_info.levelPoint"},
					}},
					{Key: "in", Value: bson.D{
						{Key: "$cond", Value: bson.A{
							bson.D{{Key: "$lt", Value: bson.A{"$$idx", bson.D{{Key: "$size", Value: "$$arr"}}}}},
							bson.D{{Key: "$arrayElemAt", Value: bson.A{"$$arr", "$$idx"}}},
							"$level",
						}},
					}},
				}},
			}},
		}}},

		// 4. Lookup creative-tool theo tool
		{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: "creative-tool"},
			{Key: "localField", Value: "tool"},
			{Key: "foreignField", Value: "index"},
			{Key: "as", Value: "tool_info"},
		}}},

		// 5. Tách tool loại t và q
		{{Key: "$addFields", Value: bson.D{
			{Key: "tool_points_t", Value: bson.D{
				{Key: "$map", Value: bson.D{
					{Key: "input", Value: bson.D{
						{Key: "$filter", Value: bson.D{
							{Key: "input", Value: "$tool_info"},
							{Key: "as", Value: "t"},
							{Key: "cond", Value: bson.D{{Key: "$eq", Value: bson.A{"$$t.type", "t"}}}},
						}},
					}},
					{Key: "as", Value: "t"},
					{Key: "in", Value: bson.D{
						{Key: "index", Value: "$$t.index"},
						{Key: "point", Value: bson.D{
							{Key: "$let", Value: bson.D{
								{Key: "vars", Value: bson.D{
									{Key: "idx", Value: bson.D{{Key: "$subtract", Value: bson.A{"$level", 1}}}},
									{Key: "arr", Value: "$$t.point"},
								}},
								{Key: "in", Value: bson.D{
									{Key: "$cond", Value: bson.A{
										bson.D{{Key: "$lt", Value: bson.A{"$$idx", bson.D{{Key: "$size", Value: "$$arr"}}}}},
										bson.D{{Key: "$arrayElemAt", Value: bson.A{"$$arr", "$$idx"}}},
										bson.D{{Key: "$arrayElemAt", Value: bson.A{"$$arr", 0}}},
									}},
								}},
							}},
						}},
					}},
				}},
			}},
			{Key: "tool_points_q", Value: bson.D{
				{Key: "$map", Value: bson.D{
					{Key: "input", Value: bson.D{
						{Key: "$filter", Value: bson.D{
							{Key: "input", Value: "$tool_info"},
							{Key: "as", Value: "t"},
							{Key: "cond", Value: bson.D{{Key: "$eq", Value: bson.A{"$$t.type", "q"}}}},
						}},
					}},
					{Key: "as", Value: "t"},
					{Key: "in", Value: bson.D{
						{Key: "index", Value: "$$t.index"},
						{Key: "point", Value: bson.D{{Key: "$arrayElemAt", Value: bson.A{"$$t.point", 0}}}},
					}},
				}},
			}},
		}}},

		// 6. Tính tool_factor và creative_process_point
		{{Key: "$addFields", Value: bson.D{
			{Key: "tool_factor", Value: bson.D{
				{Key: "$cond", Value: bson.D{
					{Key: "if", Value: bson.D{{Key: "$eq", Value: bson.A{bson.D{{Key: "$size", Value: "$tool_points_t"}}, 0}}}},
					{Key: "then", Value: 0},
					{Key: "else", Value: bson.D{
						{Key: "$reduce", Value: bson.D{
							{Key: "input", Value: bson.D{
								{Key: "$map", Value: bson.D{
									{Key: "input", Value: "$tool_points_t"},
									{Key: "as", Value: "tp"},
									{Key: "in", Value: bson.D{
										{Key: "$cond", Value: bson.A{
											bson.D{{Key: "$lt", Value: bson.A{"$$tp.point", 1}}},
											"$$tp.point",
											1,
										}},
									}},
								}},
							}},
							{Key: "initialValue", Value: 1},
							{Key: "in", Value: bson.D{{Key: "$multiply", Value: bson.A{"$$value", "$$this"}}}},
						}},
					}},
				}},
			}},
			{Key: "creative_process_point", Value: bson.D{
				{Key: "$sum", Value: bson.D{
					{Key: "$map", Value: bson.D{
						{Key: "input", Value: "$tool_points_q"},
						{Key: "as", Value: "tp"},
						{Key: "in", Value: "$$tp.point"},
					}},
				}},
			}},
		}}},

		// 7. Tính creative_task_point
		{{Key: "$addFields", Value: bson.D{
			{Key: "creative_task_point", Value: bson.D{
				{Key: "$multiply", Value: bson.A{"$performance_point", "$tool_factor"}},
			}},
		}}},

		// 8. Tính base_point
		{{Key: "$addFields", Value: bson.D{
			{Key: "base_point", Value: bson.D{
				{Key: "$subtract", Value: bson.A{"$performance_point", "$creative_task_point"}},
			}},
		}}},

		// 9. Project các field cần thiết
		{{Key: "$project", Value: bson.D{
			{Key: "task_name", Value: 1},
			{Key: "assignee_id", Value: 1},
			{Key: "team", Value: 1},
			{Key: "level", Value: 1},
			{Key: "performance_point", Value: 1},
			{Key: "tool_points_t", Value: 1},
			{Key: "tool_points_q", Value: 1},
			{Key: "tool_factor", Value: 1},
			{Key: "creative_process_point", Value: 1},
			{Key: "creative_task_point", Value: 1},
			{Key: "base_point", Value: 1},
			{Key: "done_date", Value: 1},
		}}},

		// 10. Group để tính tổng toàn bộ
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: nil},
			{Key: "total_performance_point", Value: bson.D{{Key: "$sum", Value: "$performance_point"}}},
			{Key: "total_creative_process_point", Value: bson.D{{Key: "$sum", Value: "$creative_process_point"}}},
			{Key: "total_creative_task_point", Value: bson.D{{Key: "$sum", Value: "$creative_task_point"}}},
			{Key: "total_base_point", Value: bson.D{{Key: "$sum", Value: "$base_point"}}},
			{Key: "tasks", Value: bson.D{{Key: "$push", Value: "$$ROOT"}}},
		}}},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []PerformancePointTotal
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, nil
	}

	results[0].Identifier = identifier
	return &results[0], nil
}

func GetPerformancePoints(client *mongo.Client, dbName, collectionName string, identifier string, startDate, endDate time.Time, isTeam, isWeekly bool) ([]PerformancePointTotalWithTime, error) {

	// Slide the startDate to to the EndDate using Monday
	level, err := collectionmodels.GetAllLevels(client, dbName, os.Getenv("MONGODB_COLLECTION_LEVEL"))
	if err != nil {
		return nil, err
	}

	toolList, err := collectionmodels.GetAllCreativeTools(client, dbName, os.Getenv("MONGODB_COLLECTION_CREATIVE_TOOLS"))
	if err != nil {
		return nil, err
	}

	var results []PerformancePointTotalWithTime

	if isWeekly {
		dateRanges := splitByMonday(startDate, endDate)

		for _, dateRange := range dateRanges {
			taskList, err := collectionmodels.GetCompletedTasksByDateRange(client, dbName, collectionName, isTeam, identifier, dateRange[0], dateRange[1])
			if err != nil {
				return nil, err
			}
			if len(taskList) == 0 {
				continue
			}
			per := GetPerformancePointTotals(identifier, taskList, level, toolList)
			res := PerformancePointTotalWithTime{
				StartDate:             dateRange[0],
				EndDate:               dateRange[1],
				TotalPerformancePoint: per,
			}
			results = append(results, res)
		}
		return results, nil
	}

	tasks, err := collectionmodels.GetCompletedTasksByDateRange(client, dbName, collectionName, isTeam, identifier, startDate, endDate)
	if err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return nil, nil
	}

	per := GetPerformancePointTotals(identifier, tasks, level, toolList)
	res := PerformancePointTotalWithTime{
		StartDate:             startDate,
		EndDate:               endDate,
		TotalPerformancePoint: per,
	}
	results = append(results, res)
	return results, nil
}

func GetPerformancePointTotals(identifier string, tasks []collectionmodels.CompletedTask, level []collectionmodels.Level, toolList []collectionmodels.CreativeTool) PerformancePointTotal {
	performancePointTotal := PerformancePointTotal{}

	for _, task := range tasks {
		var TaskPoint = GetPointByLevel(level, task.Team, task.Level)
		factor, sum := GetCreativeTaskFactor(toolList, task.Tool, task.Level, task.Team)
		var CreativeTaskPoint = float64(TaskPoint) * factor
		var CreativeProcessPoint = sum
		var BasePoint = float64(TaskPoint) - CreativeTaskPoint

		var CreativePoint = CreativeTaskPoint + CreativeProcessPoint
		var TotalPerformance = BasePoint + CreativePoint

		performancePointTotal.TotalBasePoint += BasePoint
		performancePointTotal.TotalCreativeProcessPoint += CreativeProcessPoint
		performancePointTotal.TotalCreativeTaskPoint += CreativeTaskPoint
		performancePointTotal.TotalPerformancePoint += TotalPerformance
	}

	performancePointTotal.Identifier = identifier
	return performancePointTotal
}

func GetPointByLevel(levels []collectionmodels.Level, team string, level int) int {

	for _, l := range levels {
		if l.Team == team {
			if level > 0 && level <= len(l.LevelPoint) {
				return l.LevelPoint[level-1]
			}
		}
	}
	return level
}

func GetCreativeTaskFactor(tools []collectionmodels.CreativeTool, inUsed []int, level int, team string) (float64, float64) {

	var toolForTeam []collectionmodels.CreativeTool
	for _, t := range tools {
		if t.Team == team {
			toolForTeam = append(toolForTeam, t)
		}
	}

	// remove all tool not inUsed by index
	var filteredTools []collectionmodels.CreativeTool
	for _, idx := range inUsed {
		for _, t := range toolForTeam {
			if t.Index == idx {
				filteredTools = append(filteredTools, t)
			}
		}
	}

	var factor float64 = 1.0
	var sum float64 = 0.0
	for _, t := range filteredTools {
		if t.Type == "t" {
			if level > 0 && level <= len(t.Point) {
				factor *= (1 - t.Point[level-1])
			}
		} else {
			if len(t.Point) > 0 {
				sum += t.Point[0]
			}
		}
	}
	if factor == 1.0 {
		factor = 0.0
	}

	return factor, sum
}

func splitByMonday(startDate, endDate time.Time) [][2]time.Time {
	var ranges [][2]time.Time

	// Normalize ngày (trunc về 00:00)
	startDate = startDate.Truncate(24 * time.Hour)
	endDate = endDate.Truncate(24 * time.Hour)

	prev := startDate

	// Tìm thứ Hai đầu tiên sau startDate
	current := startDate
	if current.Weekday() != time.Monday {
		daysUntilMonday := (int(time.Monday) - int(current.Weekday()) + 7) % 7
		if daysUntilMonday == 0 {
			daysUntilMonday = 7
		}
		current = current.AddDate(0, 0, daysUntilMonday)
	}

	for current.Before(endDate) {
		// Kết thúc là 23:59:59 thứ 2
		endOfMonday := time.Date(current.Year(), current.Month(), current.Day(), 23, 59, 59, 0, current.Location())
		ranges = append(ranges, [2]time.Time{prev, endOfMonday})
		// Bắt đầu phần tử kế tiếp là 0:00:00 thứ 3
		prev = endOfMonday.Add(time.Second)
		current = current.AddDate(0, 0, 7)
	}

	// Đoạn cuối cùng kết thúc ở endDate
	if prev.Before(endDate) || prev.Equal(endDate) {
		// Nếu endDate là thứ 2 thì kết thúc là 23:59:59 endDate
		if endDate.Weekday() == time.Monday {
			endOfMonday := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 23, 59, 59, 0, endDate.Location())
			ranges = append(ranges, [2]time.Time{prev, endOfMonday})
		} else {
			ranges = append(ranges, [2]time.Time{prev, endDate})
		}
	}

	return ranges
}
