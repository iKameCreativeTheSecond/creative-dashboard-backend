package collectionmodels

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type WeeklyTarget struct {
	ID       primitive.ObjectID `bson:"_id,omitempty"`
	Team     string             `bson:"team"`
	Point    int                `bson:"point"`
	DateFrom time.Time          `bson:"date_from"`
	DateTo   time.Time          `bson:"date_to"`
}

func GetWeeklyTargetByTeam(client *mongo.Client, dbName, collectionName string, team string, targetTime time.Time) (*WeeklyTarget, error) {

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collectionName)
	var result WeeklyTarget
	err := collection.FindOne(ctx, bson.M{"team": team, "date_from": bson.M{"$lte": targetTime}, "date_to": bson.M{"$gte": targetTime}}).Decode(&result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func UpdateWeeklyTargetByTeam(client *mongo.Client, dbName, collectionName string, target *WeeklyTarget) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collectionName)
	_, err := collection.UpdateOne(ctx, bson.M{"team": target.Team, "date_from": target.DateFrom, "date_to": target.DateTo}, bson.M{"$set": target})
	return err
}

func InsertWeeklyTarget(client *mongo.Client, dbName, collectionName string, target *WeeklyTarget) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collectionName)
	_, err := collection.InsertOne(ctx, target)
	return err
}

func DeleteWeeklyTarget(client *mongo.Client, dbName, collectionName string, team string, dateFrom, dateTo time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collectionName)
	_, err := collection.DeleteOne(ctx, bson.M{"team": team, "date_from": dateFrom, "date_to": dateTo})
	return err
}

func GetAllWeeklyTargets(client *mongo.Client, dbName, collectionName string) ([]WeeklyTarget, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collectionName)
	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	var targets []WeeklyTarget
	if err = cursor.All(ctx, &targets); err != nil {
		return nil, err
	}
	return targets, nil
}
