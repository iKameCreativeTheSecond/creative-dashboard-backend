package collectionmodels

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ProjectDetail struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	ProjectID int                `bson:"id"`
	Project   string             `bson:"project"`
	Research  string             `bson:"research"`
	Art       string             `bson:"art"`
	Concept   string             `bson:"concept"`
	Video     string             `bson:"video"`
	Pla       string             `bson:"pla"`
	UA        string             `bson:"ua"`
}

func InstertNewProjectDetailToDatabase(client *mongo.Client, dbName, collName string, projectDetail *ProjectDetail) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collName)
	_, err := collection.InsertOne(ctx, projectDetail)
	return err
}

func UpdateProjectDetailToDatabase(client *mongo.Client, dbName, collName string, projectDetail *ProjectDetail) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collName)
	_, err := collection.UpdateOne(ctx, bson.M{"project": projectDetail.Project}, bson.M{"$set": projectDetail})
	return err
}

func DeleteProjectDetailInDatabase(client *mongo.Client, dbName, collName string, project string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collName)

	_, err := collection.DeleteOne(ctx, bson.M{"project": project})
	return err
}

func GetAllProjectDetails(client *mongo.Client, dbName, collName string) ([]ProjectDetail, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collName)
	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []ProjectDetail
	for cursor.Next(ctx) {
		var projectDetail ProjectDetail
		if err := cursor.Decode(&projectDetail); err != nil {
			return nil, err
		}
		results = append(results, projectDetail)
	}
	return results, nil
}

func GetProjectDetail(client *mongo.Client, dbName, collName, projectName string) ([]ProjectDetail, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collName)
	cursor, err := collection.Find(ctx, bson.M{"project": projectName})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []ProjectDetail
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}
