package collectionmodels

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ProjectIssue struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"`
	Project        string             `bson:"project"`
	StartWeek      time.Time          `bson:"start_week"`
	TaskType       string             `bson:"task_type"`
	CompletedCount int                `bson:"completed_count"`
	Assignees      []string           `bson:"assignees"`
	Difference     int                `bson:"difference"`
	Team           string             `bson:"team,omitempty"`
	OrderCount     int                `bson:"order_count"`
	Note           string             `bson:"note,omitempty"`
}

func GetProjectIssues(client *mongo.Client, dbName, collectionName string, startTime, endTime time.Time) ([]ProjectIssue, error) {

	projects, err := GetAllOrderProjects(client, dbName, collectionName, startTime, endTime)
	if err != nil {
		return nil, err
	}
	var allIssues []ProjectIssue
	for _, proj := range projects {
		issues, err := GetProjectIssue(client, dbName, collectionName, proj.Project, startTime, endTime)
		if err != nil {
			fmt.Println("Error getting project issue for project:", proj.Project, "error:", err)
			continue
		}
		for _, issue := range issues {
			if issue.OrderCount <= 0 {
				continue
			}
			allIssues = append(allIssues, *issue)
		}
	}
	return allIssues, nil
}

func GetProjectIssue(client *mongo.Client, dbName, collectionName string, project string, startTime, endTime time.Time) ([]*ProjectIssue, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collectionName)

	matchStage := bson.D{
		{Key: "start_week", Value: bson.D{
			{Key: "$gte", Value: startTime},
			{Key: "$lte", Value: endTime},
		}},
	}
	if project != "" {
		matchStage = append(matchStage, bson.E{Key: "project", Value: project})
	}

	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: matchStage}},
		{{Key: "$project", Value: bson.D{
			{Key: "project", Value: 1},
			{Key: "start_week", Value: 1},
			{Key: "orders", Value: bson.A{
				bson.D{{Key: "task_type", Value: "art_cpp"}, {Key: "order_count", Value: "$art_cpp"}},
				bson.D{{Key: "task_type", Value: "art_icon"}, {Key: "order_count", Value: "$art_icon"}},
				bson.D{{Key: "task_type", Value: "art_banner"}, {Key: "order_count", Value: "$art_banner"}},
				bson.D{{Key: "task_type", Value: "playable"}, {Key: "order_count", Value: "$playable"}},
				bson.D{{Key: "task_type", Value: "video"}, {Key: "order_count", Value: "$video"}},
			}},
		}}},
		{{Key: "$unwind", Value: "$orders"}},
		{{Key: "$lookup", Value: bson.D{
			{Key: "from", Value: "completed-task"},
			{Key: "let", Value: bson.D{
				{Key: "projectName", Value: "$project"},
				{Key: "taskType", Value: "$orders.task_type"},
			}},
			{Key: "pipeline", Value: mongo.Pipeline{
				{{Key: "$match", Value: bson.D{
					{Key: "$expr", Value: bson.D{
						{Key: "$and", Value: bson.A{
							bson.D{{Key: "$eq", Value: bson.A{"$project", "$$projectName"}}},
							bson.D{{Key: "$eq", Value: bson.A{"$task_type", "$$taskType"}}},
							bson.D{{Key: "$gte", Value: bson.A{"$done_date", startTime}}},
							bson.D{{Key: "$lte", Value: bson.A{"$done_date", endTime}}},
						}},
					}},
				}}},
			}},
			{Key: "as", Value: "completed_tasks"},
		}}},
		{{Key: "$addFields", Value: bson.D{
			{Key: "completed_count", Value: bson.D{{Key: "$size", Value: "$completed_tasks"}}},
			{Key: "assignees", Value: bson.D{{Key: "$setUnion", Value: bson.A{"$completed_tasks.assignee_id"}}}},
		}}},
		{{Key: "$project", Value: bson.D{
			{Key: "_id", Value: 0},
			{Key: "project", Value: 1},
			{Key: "start_week", Value: 1},
			{Key: "task_type", Value: "$orders.task_type"},
			{Key: "order_count", Value: "$orders.order_count"},
			{Key: "completed_count", Value: 1},
			{Key: "assignees", Value: 1},
			{Key: "difference", Value: bson.D{{Key: "$subtract", Value: bson.A{"$completed_count", "$orders.order_count"}}}},
			{Key: "note", Value: bson.D{{Key: "$cond", Value: bson.A{
				bson.D{{Key: "$gt", Value: bson.A{"$completed_count", "$orders.order_count"}}},
				"OVER",
				bson.D{{Key: "$cond", Value: bson.A{
					bson.D{{Key: "$lt", Value: bson.A{"$completed_count", "$orders.order_count"}}},
					"UNDER",
					"MATCH",
				}}},
			}}}},
		}}},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []*ProjectIssue
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	projectDetails, err := GetProjectDetail(client, dbName, os.Getenv("MONGODB_COLLECTION_PROJECT_DETAIL"), project)
	if err == nil && len(projectDetails) > 0 {

		detailMap := make(map[string]ProjectDetail)
		for _, detail := range projectDetails {
			detailMap[detail.Project] = detail
		}
		for i, issue := range results {
			if detail, exists := detailMap[issue.Project]; exists {
				switch issue.TaskType {
				case "art_cpp", "art_icon", "art_banner":
					results[i].Team = "Art"
					if len(issue.Assignees) == 0 {
						issue.Assignees = []string{detail.Art}
					}
				case "playable":
					results[i].Team = "PLA"
					if len(issue.Assignees) == 0 {
						issue.Assignees = []string{detail.Pla}
					}
				case "video":
					results[i].Team = "Video"
					if len(issue.Assignees) == 0 {
						issue.Assignees = []string{detail.Video}
					}
				}
			}
		}

	}

	return results, nil
}

func InsertProjectIssue(client *mongo.Client, dbName, collectionName string, issue ProjectIssue) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collectionName)
	_, err := collection.InsertOne(ctx, issue)
	return err
}

func InsertProjectIssues(client *mongo.Client, dbName, collectionName string, issues []ProjectIssue) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collectionName)
	var docs []interface{}
	for _, issue := range issues {
		docs = append(docs, issue)
	}
	_, err := collection.InsertMany(ctx, docs)
	return err
}

func DeleteProjectIssue(client *mongo.Client, dbName, collectionName string, id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collectionName)
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	_, err = collection.DeleteOne(ctx, bson.M{"_id": objID})
	return err
}

func UpdateProjectIssue(client *mongo.Client, dbName, collectionName string, issue ProjectIssue) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collectionName)
	_, err := collection.UpdateOne(ctx, bson.M{"_id": issue.ID.String()}, bson.M{"$set": issue})
	return err
}

func GetProjectIssueFromBD(client *mongo.Client, dbName, collectionName string, startTime, endTime time.Time) (*[]ProjectIssue, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collectionName)
	filter := bson.M{
		"start_week": bson.M{
			"$gte": startTime,
			"$lte": endTime,
		},
	}
	cursor, err := collection.Find(ctx, filter)
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
