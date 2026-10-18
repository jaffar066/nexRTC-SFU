package media

import "testing"

func TestProducerLifecycleDistinguishesMuteAndPause(t *testing.T) {
	producer := mustProducer(t, "producer-a", "participant-a", KindAudio)
	if err := producer.SetMuted(true); err == nil {
		t.Fatal("new producer muted before activation")
	}
	if err := producer.Activate(); err != nil {
		t.Fatal(err)
	}
	if err := producer.SetMuted(true); err != nil {
		t.Fatal(err)
	}
	if producer.State() != StateMuted {
		t.Fatalf("muted producer state = %s", producer.State())
	}
	if err := producer.Pause(); err == nil {
		t.Fatal("muted producer also transitioned to paused")
	}
	if err := producer.SetMuted(false); err != nil {
		t.Fatal(err)
	}
	if err := producer.Pause(); err != nil {
		t.Fatal(err)
	}
	if producer.State() != StatePaused {
		t.Fatalf("paused producer state = %s", producer.State())
	}
	if err := producer.Resume(); err != nil {
		t.Fatal(err)
	}
	if err := producer.BeginClose(); err != nil {
		t.Fatal(err)
	}
	if err := producer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := producer.Activate(); err == nil {
		t.Fatal("closed producer transitioned back to active")
	}
}

func TestConsumerLifecycleIsIndependent(t *testing.T) {
	consumer := mustConsumer(t, "consumer-a", "producer-a")
	if err := consumer.Activate(); err != nil {
		t.Fatal(err)
	}
	if err := consumer.Pause(); err != nil {
		t.Fatal(err)
	}
	if err := consumer.Pause(); err != nil {
		t.Fatalf("repeated pause should be idempotent: %v", err)
	}
	if err := consumer.Resume(); err != nil {
		t.Fatal(err)
	}
	if consumer.State() != StateActive {
		t.Fatalf("state = %s, want ACTIVE", consumer.State())
	}
	if err := consumer.BeginClose(); err != nil {
		t.Fatal(err)
	}
	if err := consumer.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestMediaConstructorsValidateIDsAndKinds(t *testing.T) {
	if _, err := NewProducer("", "participant", KindAudio); err == nil {
		t.Fatal("producer accepted an empty ID")
	}
	if _, err := NewProducer("producer", "", KindAudio); err == nil {
		t.Fatal("producer accepted an empty participant ID")
	}
	if _, err := NewProducer("producer", "participant", "data"); err == nil {
		t.Fatal("producer accepted an unsupported kind")
	}
	if _, err := NewConsumer("", "producer"); err == nil {
		t.Fatal("consumer accepted an empty ID")
	}
	if _, err := NewConsumer("consumer", ""); err == nil {
		t.Fatal("consumer accepted an empty producer ID")
	}
}

func mustProducer(t *testing.T, id, participantID string, kind Kind) *Producer {
	t.Helper()
	producer, err := NewProducer(id, participantID, kind)
	if err != nil {
		t.Fatal(err)
	}
	return producer
}

func mustConsumer(t *testing.T, id, producerID string) *Consumer {
	t.Helper()
	consumer, err := NewConsumer(id, producerID)
	if err != nil {
		t.Fatal(err)
	}
	return consumer
}
