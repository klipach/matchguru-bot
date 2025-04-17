PROJECT_ID=match-guru-0iqc9r
FUNCTION_NAME=bot

deploy:
	gcloud functions deploy $(FUNCTION_NAME) \
	--gen2 \
	--region=us-central1 \
	--runtime=go123 \
	--trigger-http \
	--project=$(PROJECT_ID) \
	--allow-unauthenticated \
	--entry-point=Bot \
	--source .

get_function_url:
	gcloud functions describe $(FUNCTION_NAME) \
	--project=$(PROJECT_ID) \
	--format="value(url)"

firebase:
	firebase deploy --only hosting

get_token: # generate a valid JWT token for my user
	go run cmd/gentoken/main.go -apikey AIzaSyASGNK8O7_LhmipjLKhTUJpqObdZ_gm_Fc  -uid QfaKDmbvXfMGHfDvk4zUwVIrRSW2

