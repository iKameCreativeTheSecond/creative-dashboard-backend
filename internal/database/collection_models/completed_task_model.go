package collectionmodels

import (
	"time"

	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type CompletedTask struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	TaskID     string             `bson:"id"`
	TaskName   string             `bson:"task_name"`
	AssigneeID string             `bson:"assignee_id"`
	Tool       []int              `bson:"tool"`
	Level      int                `bson:"level"`
	TaskType   string             `bson:"task_type"`
	Project    string             `bson:"project"`
	Team       string             `bson:"team"`
	DoneDate   time.Time          `bson:"done_date"`
}

//	{
//	  done_date:
//	  {
//	    $gte: new Date("2025-09-01T00:00:00.000Z"),
//	    $lte: new Date("2025-09-02T23:59:59.999Z")
//	  }
//	}
//

func InsertCompletedTaskToDataBase(client *mongo.Client, dbName, collectionName string, tasks []*CompletedTask) error {
	collection := client.Database(dbName).Collection(collectionName)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var docs []interface{}
	for _, task := range tasks {
		docs = append(docs, task)
	}
	_, err := collection.InsertMany(ctx, docs)
	return err
}

func GetCompletedTasksByDateRange(client *mongo.Client, dbName, collectionName string, isTeam bool, identifier string, startDate, endDate time.Time) ([]CompletedTask, error) {
	collection := client.Database(dbName).Collection(collectionName)

	var indentifierKey string
	if isTeam {
		indentifierKey = "team"
	} else {
		indentifierKey = "assignee_id"
	}

	filter := bson.M{
		indentifierKey: identifier,
		"done_date": bson.M{
			"$gte": startDate,
			"$lte": endDate,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var tasks []CompletedTask
	for cursor.Next(ctx) {
		var task CompletedTask
		if err := cursor.Decode(&task); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	if err := cursor.Err(); err != nil {
		return nil, err
	}
	return tasks, nil
}
