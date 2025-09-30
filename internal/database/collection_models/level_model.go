package collectionmodels

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type Level struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	Team       string             `bson:"team"`
	LevelPoint []int              `bson:"levelPoint"`
}

// Add to the databse a new level for a team
func AddNewLevelForTeam(client *mongo.Client, dbName, collectionName string, level *Level) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collectionName)
	_, err := collection.InsertOne(ctx, level)
	return err
}

// Update the level points for a team
func UpdateLevelPointsForTeam(client *mongo.Client, dbName, collectionName string, level *Level) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collectionName)
	_, err := collection.UpdateOne(ctx,
		bson.M{"team": level.Team},
		bson.M{"$set": bson.M{"levelPoint": level.LevelPoint}},
	)
	return err
}

// Get the level points for a team
func GetLevelPointsForTeam(client *mongo.Client, dbName, collectionName, team string) (*Level, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collectionName)
	var level Level
	err := collection.FindOne(ctx, bson.M{"team": team}).Decode(&level)
	if err != nil {
		return nil, err
	}
	return &level, nil
}

func DeleteLevelForTeam(client *mongo.Client, dbName, collectionName, team string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collectionName)
	_, err := collection.DeleteOne(ctx, bson.M{"team": team})
	return err
}

func GetAllLevels(client *mongo.Client, dbName, collectionName string) ([]Level, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collectionName)
	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	var levels []Level
	if err := cursor.All(ctx, &levels); err != nil {
		return nil, err
	}
	return levels, nil
}
