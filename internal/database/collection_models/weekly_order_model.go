package collectionmodels

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type WeeklyOrder struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"`
	StartWeek time.Time          `bson:"start_week"`
	Goal      string             `bson:"goal"`
	Strategy  string             `bson:"strategy"`
	Project   string             `bson:"project"`
	CPP       int                `bson:"art_cpp"`
	Icon      int                `bson:"art_icon"`
	Banner    int                `bson:"art_banner"`
	PLA       int                `bson:"playable"`
	Video     int                `bson:"video"`
}

type WeeklyOrderProject struct {
	Project string `bson:"project" json:"project"`
}

func InsertWeeklyOrder(client *mongo.Client, dbName, collName string, order *WeeklyOrder) (primitive.ObjectID, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collName)
	res, err := collection.InsertOne(ctx, order)
	id := res.InsertedID.(primitive.ObjectID)
	return id, err
}

func UpdateWeeklyOrder(client *mongo.Client, dbName, collName string, order *WeeklyOrder) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collName)
	// base on start_week and project to update
	_, err := collection.UpdateOne(ctx, bson.M{"start_week": order.StartWeek, "project": order.Project}, bson.M{"$set": order})
	return err
}

func GetWeeklyOrders(client *mongo.Client, dbName, collName string, startWeek []time.Time, project []string) ([]*WeeklyOrder, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collName)
	filter := bson.M{
		"start_week": bson.M{"$in": startWeek},
		"project":    bson.M{"$in": project},
	}
	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []*WeeklyOrder
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func DeleteWeeklyOrder(client *mongo.Client, dbName, collName string, startWeek time.Time, project string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collName)
	_, err := collection.DeleteOne(ctx, bson.M{"start_week": startWeek, "project": project})
	return err
}

func DeleteWeeklyOrderByID(client *mongo.Client, dbName, collName string, id primitive.ObjectID) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collName)
	_, err := collection.DeleteOne(ctx, bson.M{"_id": id})
	return err
}

func GetAllWeeklyOrders(client *mongo.Client, dbName, collName string) ([]*WeeklyOrder, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	collection := client.Database(dbName).Collection(collName)
	cursor, err := collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)
	var results []*WeeklyOrder
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

func GetAllOrderProjects(client *mongo.Client, dbName, collName string, startDate, endDate time.Time) ([]*WeeklyOrderProject, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := client.Database(dbName).Collection(collName)
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.D{{Key: "start_week", Value: bson.D{{Key: "$gte", Value: startDate}, {Key: "$lte", Value: endDate}}}}}},
		{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$project"}}}},
		{{Key: "$project", Value: bson.D{{Key: "_id", Value: 0}, {Key: "project", Value: "$_id"}}}},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []*WeeklyOrderProject
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}
