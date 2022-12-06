package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/lidofinance/wc_sig_listener/internal/entity"
	"github.com/lidofinance/wc_sig_listener/internal/env"
	"github.com/lidofinance/wc_sig_listener/internal/kafka"
	"github.com/sirupsen/logrus"
	types "github.com/wealdtech/go-eth2-types/v2"
	"os"
)

const SignatureReconstructedEvent = "signature_reconstructed"

type (
	MessageDTO struct {
		ID            string `json:"id"`
		DkgRoundID    string `json:"dkg_round_id"`
		Offset        uint64 `json:"offset"`
		Event         string `json:"event"`
		Data          []byte `json:"data"`
		Signature     []byte `json:"signature"`
		SenderAddr    string `json:"sender"`
		RecipientAddr string `json:"recipient"`
		ValidatorID   uint64 `json:"validator_id"`
	}

	ReconstructedSignature struct {
		File      string
		BatchID   string
		MessageID string
		// SrcPayload - signingroot of signature BLSToExecutionChange
		SrcPayload []byte
		// Signature of BLSToExecutionChange
		Signature  []byte
		Username   string
		DKGRoundID string
		// Special field with additional info for "sign baked data" routine
		ValIdx int64
	}
)

func init() {
	types.InitBLS()
}

// sort output.csv --output=res.csv -u
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, envErr := env.ReadEnv()
	if envErr != nil {
		println("Read env error:", envErr.Error())

		os.Exit(1)
	}

	log := logrus.StandardLogger()
	log.SetFormatter(&logrus.TextFormatter{})
	f, err := os.OpenFile(`./log.txt`, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0655)
	if err != nil {
		log.Fatal(fmt.Errorf(`could not create log.txt %w`, err))
	}
	log.SetOutput(f)

	output, err := os.OpenFile(`./output.csv`, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0655)
	if err != nil {
		log.Fatal(fmt.Errorf(`could not create output.csv %w`, err))
	}

	executionAddress, err := hex.DecodeString(cfg.ExecutionAddress)
	if err != nil {
		log.Fatal(fmt.Errorf(`could not decode execution address %w`, err))
	}
	var execAdr [32]byte
	copy(execAdr[:], executionAddress)

	agrBlsPub, err := hex.DecodeString(cfg.AgrPubKey)
	if err != nil {
		log.Fatal(fmt.Errorf(`could not decode aggregated_bls_pub_key %w`, err))
	}
	var agrPub [48]byte
	copy(agrPub[:], agrBlsPub)

	blsAg, err := types.BLSPublicKeyFromBytes(agrPub[:])
	if err != nil {
		log.Fatal(fmt.Errorf(`could not set bls pub key %w`, err))
	}

	fkVer, err := hex.DecodeString(cfg.ForkVersion)
	if err != nil {
		log.Fatal(fmt.Errorf(`could not decode forkVersion %w`, err))
	}
	var forkVersion [4]byte
	copy(forkVersion[:], fkVer)

	kafkaDialer, err := kafka.NewDialer(cfg.KafkaConfig.Creds, `45s`)
	if err != nil {
		log.Fatal(fmt.Errorf(`could not create instance of kafka dialer %w`, err))
	}

	kafkaClient := kafka.NewReaderConfig(
		kafka.WithBrokers([]string{cfg.KafkaConfig.Host}),
		// kafka.WithGroupID(cfg.KafkaConfig.GroupID),
		kafka.WithTopic(cfg.KafkaConfig.Topic),
		kafka.WithDialer(kafkaDialer),
	)

	defer func() {
		if closeErr := kafkaClient.Close(); err != nil {
			log.Fatal(fmt.Errorf(`could not close kafka connection %w`, closeErr))
		}
	}()

	var counter uint64
	result := make(map[int64]entity.BLSToExecutionChange)
	for {
		m, err := kafkaClient.ReadMessage(ctx)
		if err != nil {
			log.Fatal(fmt.Errorf("failed to read message: %w", err))
		}

		var message MessageDTO
		if err := json.Unmarshal(m.Value, &message); err != nil {
			if commitErr := kafkaClient.CommitMessages(ctx, m); commitErr != nil {
				log.Error(fmt.Errorf("failed to commit a message %w\n", commitErr))
			}

			return
		}

		if message.Event != SignatureReconstructedEvent {
			continue
		}

		var payload []ReconstructedSignature
		if err := json.Unmarshal(message.Data, &payload); err != nil {
			log.Error(fmt.Errorf("failed to commit a message %w\n", err))

			return
		}

		for _, p := range payload {
			if _, found := result[p.ValIdx]; found {
				continue
			}

			var signingRoot [32]byte
			copy(signingRoot[:], p.SrcPayload)

			obj := entity.BLSToExecutionChange{
				ValidatorIndex:     uint64(p.ValIdx),
				Pubkey:             agrPub,
				ToExecutionAddress: execAdr,
			}

			result[p.ValIdx] = obj

			rootActual, err := obj.GetRoot(forkVersion)
			if err != nil {
				log.WithField(`validator_idx`, p.ValIdx).Error(fmt.Errorf("could not get signing root %w\n", err))

				return
			}

			if hex.EncodeToString(rootActual[:]) != hex.EncodeToString(signingRoot[:]) {
				log.WithField(`validator_idx`, p.ValIdx).Error(fmt.Errorf("expected signing root of blsExecutionChange is wrong %w\n", err))

				return
			}

			blsSignature, err := types.BLSSignatureFromBytes(p.Signature)
			if err != nil {
				log.WithField(`validator_idx`, p.ValIdx).Error(fmt.Errorf("could not reconstruct bls signarure %w\n", err))
				return
			}

			if cfg.CheckSignature {
				if !blsSignature.Verify(rootActual[:], blsAg) {
					log.WithField(`validator_idx`, p.ValIdx).Error(fmt.Errorf("signature of BLSToExecutionChange is wrong %w\n", err))
					return
				}
			}

			counter++
			line := fmt.Sprintf(`%d,%s,%s`, p.ValIdx, hex.EncodeToString(p.Signature), hex.EncodeToString(signingRoot[:]))

			_, _ = fmt.Fprintln(output, line)

			percentageDone := float64(counter) * 100 / float64(18632)
			log.Println(fmt.Sprintf(`Saved. offset %d ValidatorID: %d, signature %s: counter: %d/18632. %.2f%%`, m.Offset, p.ValIdx, hex.EncodeToString(p.Signature), counter, percentageDone))

			println(fmt.Sprintf(`ValidatorID: %d Saving: %d/18632. %.2f%%`, p.ValIdx, counter, percentageDone))
		}
	}
}
