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

type PerformancePoint struct {
	StartWeek                 time.Time `bson:"_id"`
	TotalPerformancePoint     float64   `bson:"total_performance_point"`
	TotalCreativeProcessPoint float64   `bson:"total_creative_process_point"`
	TotalCreativeTaskPoint    float64   `bson:"total_creative_task_point"`
	TotalBasePoint            float64   `bson:"total_base_point"`
	Identifier                string    `bson:"identifier"`
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
		// 1. Match theo assignee_id và done_date
		{{
			Key: "$match", Value: bson.D{
				{Key: identifierKey, Value: identifier},
				{Key: "done_date", Value: bson.D{
					{Key: "$gte", Value: startDate},
					{Key: "$lte", Value: endDate},
				}},
			},
		}},

		// 2. Add week_start (truncate theo tuần, bắt đầu Monday)
		{{
			Key: "$addFields", Value: bson.D{
				{Key: "week_start", Value: bson.D{
					{Key: "$dateTrunc", Value: bson.D{
						{Key: "date", Value: "$done_date"},
						{Key: "unit", Value: "week"},
						{Key: "binSize", Value: 1},
						{Key: "startOfWeek", Value: "monday"},
					}},
				}},
			},
		}},

		// 3. Lookup level theo team
		{{
			Key: "$lookup", Value: bson.D{
				{Key: "from", Value: "level"},
				{Key: "localField", Value: "team"},
				{Key: "foreignField", Value: "team"},
				{Key: "as", Value: "level_info"},
			},
		}},
		{{Key: "$unwind", Value: "$level_info"}},

		// 4. Tính performance_point
		{{
			Key: "$addFields", Value: bson.D{
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
			},
		}},

		// 5. Lookup creative-tool
		{{
			Key: "$lookup", Value: bson.D{
				{Key: "from", Value: "creative-tool"},
				{Key: "localField", Value: "tool"},
				{Key: "foreignField", Value: "tool_name"},
				{Key: "as", Value: "tool_info"},
			},
		}},

		// 6. Add tool_points_t và tool_points_q
		{{
			Key: "$addFields", Value: bson.D{
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
							{Key: "tool_name", Value: "$$t.tool_name"},
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
							{Key: "tool_name", Value: "$$t.tool_name"},
							{Key: "point", Value: bson.D{{Key: "$arrayElemAt", Value: bson.A{"$$t.point", 0}}}},
						}},
					}},
				}},
			},
		}},

		// 7. Add tool_factor và creative_process_point
		{{
			Key: "$addFields", Value: bson.D{
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
			},
		}},

		// 8. Add creative_task_point
		{{
			Key: "$addFields", Value: bson.D{
				{Key: "creative_task_point", Value: bson.D{
					{Key: "$multiply", Value: bson.A{"$performance_point", "$tool_factor"}},
				}},
			},
		}},

		// 9. Add base_point
		{{
			Key: "$addFields", Value: bson.D{
				{Key: "base_point", Value: bson.D{
					{Key: "$add", Value: bson.A{
						bson.D{{Key: "$subtract", Value: bson.A{"$performance_point", "$creative_task_point"}}},
						"$creative_process_point",
					}},
				}},
			},
		}},

		// 10. Project
		{{
			Key: "$project", Value: bson.D{
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
			},
		}},

		// 11. Group theo tuần
		{{
			Key: "$group", Value: bson.D{
				{Key: "_id", Value: "$week_start"},
				{Key: "total_performance_point", Value: bson.D{{Key: "$sum", Value: "$performance_point"}}},
				{Key: "total_creative_process_point", Value: bson.D{{Key: "$sum", Value: "$creative_process_point"}}},
				{Key: "total_creative_task_point", Value: bson.D{{Key: "$sum", Value: "$creative_task_point"}}},
				{Key: "total_base_point", Value: bson.D{{Key: "$sum", Value: "$base_point"}}},
				{Key: "tasks", Value: bson.D{{Key: "$push", Value: "$$ROOT"}}},
			},
		}},

		// 12. Sort
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
