.PHONY: run_emulator
run_emulator:
	# export FIRESTORE_EMULATOR_HOST=localhost:8123
	gcloud beta emulators firestore start --host-port localhost:8123
