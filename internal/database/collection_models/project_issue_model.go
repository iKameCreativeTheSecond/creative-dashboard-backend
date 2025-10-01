package collectionmodels

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ProjectIssue struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"`
	Project        string             `bson:"project"`
	StartWeek      time.Time          `bson:"start_week"`
	CompletedCount int                `bson:"completed_count"`
	Assignees      []string           `bson:"assignees"`
	Difference     int                `bson:"difference"`
	Team           string             `bson:"team"`
	OrderCount     int                `bson:"order_count"`
}

func GetProjectIssues(client *mongo.Client, dbName, collectionName string, startTime, endTime time.Time) (*[]ProjectIssue, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collectionName)

	pipeline := mongo.Pipeline{
		// 1. Filter by date range
		{{Key: "$match", Value: bson.D{
			{Key: "start_week", Value: bson.D{
				{Key: "$gte", Value: startTime},
				{Key: "$lte", Value: endTime},
			}},
		}}},
		// 2. Build orders array
		{{Key: "$project", Value: bson.D{
			{Key: "project", Value: 1},
			{Key: "start_week", Value: 1},
			{Key: "orders", Value: bson.A{
				bson.D{{Key: "team", Value: "Art Creative"}, {Key: "count", Value: bson.D{{Key: "$add", Value: bson.A{"$art_cpp", "$art_icon", "$art_banner"}}}}},
				bson.D{{Key: "team", Value: "Video Creative"}, {Key: "count", Value: "$video"}},
				bson.D{{Key: "team", Value: "PLA Creative"}, {Key: "count", Value: "$playable"}},
			}},
		}}},
		// 3. Unwind orders
		{{Key: "$unwind", Value: "$orders"}},
		// 4. Lookup completed-task
		{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: "completed-task"},
			{Key: "let", Value: bson.D{
				{Key: "proj", Value: "$project"},
				{Key: "team", Value: "$orders.team"},
			}},
			{Key: "pipeline", Value: mongo.Pipeline{
				{{Key: "$match", Value: bson.D{
					{Key: "$expr", Value: bson.D{
						{Key: "$and", Value: bson.A{
							bson.D{{Key: "$eq", Value: bson.A{"$project", "$$proj"}}},
							bson.D{{Key: "$eq", Value: bson.A{"$team", "$$team"}}},
						}},
					}},
				}}},
				{{Key: "$group", Value: bson.D{
					{Key: "_id", Value: "$assignee_id"},
					{Key: "completed_count", Value: bson.D{{Key: "$sum", Value: 1}}},
				}}},
			}},
			{Key: "as", Value: "completed"},
		}}},
		// 5. Lookup fallback from project-details
		{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: "project-details"},
			{Key: "let", Value: bson.D{
				{Key: "proj", Value: "$project"},
				{Key: "team", Value: "$orders.team"},
			}},
			{Key: "pipeline", Value: mongo.Pipeline{
				{{Key: "$match", Value: bson.D{
					{Key: "$expr", Value: bson.D{
						{Key: "$eq", Value: bson.A{"$project", "$$proj"}},
					}},
				}}},
				{{Key: "$project", Value: bson.D{
					{Key: "assignee", Value: bson.D{
						{Key: "$switch", Value: bson.D{
							{Key: "branches", Value: bson.A{
								bson.D{
									{Key: "case", Value: bson.D{{Key: "$eq", Value: bson.A{"$$team", "Art Creative"}}}},
									{Key: "then", Value: "$art"},
								},
								bson.D{
									{Key: "case", Value: bson.D{{Key: "$eq", Value: bson.A{"$$team", "Video Creative"}}}},
									{Key: "then", Value: "$video"},
								},
								bson.D{
									{Key: "case", Value: bson.D{{Key: "$eq", Value: bson.A{"$$team", "PLA Creative"}}}},
									{Key: "then", Value: "$pla"},
								},
							}},
							{Key: "default", Value: nil},
						}},
					}},
				}}},
			}},
			{Key: "as", Value: "fallback"},
		}}},
		// 6. Add completed_count + assignees (fallback if empty)
		{{Key: "$addFields", Value: bson.D{
			{Key: "completed_count", Value: bson.D{{Key: "$sum", Value: "$completed.completed_count"}}},
			{Key: "assignees", Value: bson.D{
				{Key: "$cond", Value: bson.D{
					{Key: "if", Value: bson.D{{Key: "$gt", Value: bson.A{bson.D{{Key: "$size", Value: "$completed"}}, 0}}}},
					{Key: "then", Value: "$completed._id"},
					{Key: "else", Value: bson.D{
						{Key: "$map", Value: bson.D{
							{Key: "input", Value: "$fallback"},
							{Key: "as", Value: "f"},
							{Key: "in", Value: "$$f.assignee"},
						}},
					}},
				}},
			}},
		}}},
		// 7. Calculate difference
		{{Key: "$addFields", Value: bson.D{
			{Key: "difference", Value: bson.D{
				{Key: "$subtract", Value: bson.A{"$orders.count", bson.D{{Key: "$ifNull", Value: bson.A{"$completed_count", 0}}}}},
			}},
		}}},
		// 8. Only keep where difference > 0
		{{Key: "$match", Value: bson.D{
			{Key: "difference", Value: bson.D{{Key: "$gt", Value: 0}}},
		}}},
		// 9. Final projection
		{{Key: "$project", Value: bson.D{
			{Key: "project", Value: 1},
			{Key: "start_week", Value: 1},
			{Key: "team", Value: "$orders.team"},
			{Key: "order_count", Value: "$orders.count"},
			{Key: "completed_count", Value: 1},
			{Key: "difference", Value: 1},
			{Key: "assignees", Value: 1},
		}}},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []ProjectIssue
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	return &results, nil
}
