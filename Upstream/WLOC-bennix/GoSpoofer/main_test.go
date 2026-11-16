package main

import (
	"encoding/hex"
	"testing"

	pb "golocationspoofer/pb"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

func TestDeserializeRealRequest(t *testing.T) {
	requestBytes, _ := hex.DecodeString("0001000a656e2d3030315f3030310013636f6d2e6170706c652e6c6f636174696f6e64000a32362e322e3233433535000000020000000002000000")

	arpc := ArpcDeserialize(requestBytes)
	if arpc == nil {
		t.Fatal("Failed to deserialize ARPC")
	}

	t.Logf("ARPC payload size: %d bytes", len(arpc.Payload))
	t.Logf("Version: %s", arpc.Version)
	t.Logf("Locale: %s", arpc.Locale)
	t.Logf("AppIdentifier: %s", arpc.AppIdentifier)
	t.Logf("OsVersion: %s", arpc.OsVersion)
	t.Logf("FunctionId: %d", arpc.FunctionId)

	wloc := &pb.AppleWLoc{}
	if err := proto.Unmarshal(arpc.Payload, wloc); err != nil {
		t.Fatalf("Failed to unmarshal protobuf: %v", err)
	}

	t.Logf("Number of wifi devices: %d", len(wloc.WifiDevices))

	if len(wloc.WifiDevices) == 0 {
		t.Logf("WifiDevices slice is empty or nil")
	} else {
		for i, device := range wloc.WifiDevices {
			t.Logf("Device %d: BSSID=%s", i, device.GetBssid())
		}
	}
}

func TestFullRoundTrip(t *testing.T) {
	requestBytes, _ := hex.DecodeString("0001000a656e2d3030315f3030310013636f6d2e6170706c652e6c6f636174696f6e64000a32362e322e3233433535000000020000000002000000")

	arpc := ArpcDeserialize(requestBytes)
	if arpc == nil {
		t.Fatal("Failed to deserialize ARPC")
	}

	wloc := &pb.AppleWLoc{}
	if err := proto.Unmarshal(arpc.Payload, wloc); err != nil {
		t.Fatalf("Failed to unmarshal protobuf: %v", err)
	}

	if len(wloc.WifiDevices) == 0 {
		wloc.WifiDevices = []*pb.WifiDevice{{Bssid: "aa:bb:cc:dd:ee:ff"}}
	}

	lat := IntFromCoord(51.510420)
	lon := IntFromCoord(-3.218306)
	horizontalAccuracy := int64(39)
	verticalAccuracy := int64(1000)
	altitude := int64(530)
	unknownValue4 := int64(3)
	motionActivityType := int64(63)
	motionActivityConfidence := int64(467)

	for i, device := range wloc.WifiDevices {
		t.Logf("Spoofing BSSID: %s (index %d)", device.GetBssid(), i)
		if device.Location == nil {
			device.Location = &pb.Location{}
		}
		device.Location.Latitude = &lat
		device.Location.Longitude = &lon
		device.Location.HorizontalAccuracy = &horizontalAccuracy
		device.Location.VerticalAccuracy = &verticalAccuracy
		device.Location.Altitude = &altitude
		device.Location.UnknownValue4 = &unknownValue4
		device.Location.MotionActivityType = &motionActivityType
		device.Location.MotionActivityConfidence = &motionActivityConfidence
	}

	wloc.NumCellResults = nil
	wloc.NumWifiResults = nil
	wloc.DeviceType = nil

	initialBytes, _ := hex.DecodeString("0001000000010000")
	responseBytes, err := SerializeProto(wloc, initialBytes)
	if err != nil {
		t.Fatalf("Failed to serialize: %v", err)
	}

	t.Logf("Request had %d wifi devices", len(wloc.WifiDevices))
	t.Logf("Response bytes: %x", responseBytes)
	t.Logf("Response length: %d bytes", len(responseBytes))

	if responseBytes[0] != 0x00 || responseBytes[1] != 0x01 {
		t.Errorf("First 2 bytes should be 0x0001, got: %x", responseBytes[:2])
	}

	payloadLen := int(responseBytes[8])<<8 | int(responseBytes[9])
	t.Logf("Embedded payload length: %d bytes", payloadLen)
	t.Logf("Total response length: %d bytes", len(responseBytes))

	if payloadLen != len(responseBytes)-10 {
		t.Errorf("Payload length mismatch: embedded=%d, actual=%d", payloadLen, len(responseBytes)-10)
	}
}

func TestRewriteWifiAndCellLocations(t *testing.T) {
	wifi := protowire.AppendTag(nil, 1, protowire.BytesType)
	wifi = protowire.AppendString(wifi, "aa:bb:cc:dd:ee:ff")
	cell := protowire.AppendTag(nil, 1, protowire.VarintType)
	cell = protowire.AppendVarint(cell, 460)

	payload := protowire.AppendTag(nil, 2, protowire.BytesType)
	payload = protowire.AppendBytes(payload, wifi)
	payload = protowire.AppendTag(payload, 22, protowire.BytesType)
	payload = protowire.AppendBytes(payload, cell)
	payload = protowire.AppendTag(payload, 24, protowire.BytesType)
	payload = protowire.AppendBytes(payload, cell)
	payload = protowire.AppendTag(payload, 3, protowire.VarintType)
	payload = protowire.AppendVarint(payload, 1)
	payload = protowire.AppendTag(payload, 4, protowire.VarintType)
	payload = protowire.AppendVarint(payload, 1)

	lat := IntFromCoord(39.9042)
	lon := IntFromCoord(116.4074)
	rewritten, modified := rewriteAppleWLocCoords(payload, lat, lon)
	if modified != 24 {
		t.Fatalf("expected 24 rewritten location fields, got %d", modified)
	}

	rootFields := map[protowire.Number]int{}
	for rest := rewritten; len(rest) > 0; {
		num, typ, tagLen := protowire.ConsumeTag(rest)
		if tagLen < 0 {
			t.Fatal("invalid rewritten protobuf tag")
		}
		n := protowire.ConsumeFieldValue(num, typ, rest[tagLen:])
		if n < 0 {
			t.Fatal("invalid rewritten protobuf value")
		}
		rootFields[num]++
		rest = rest[tagLen+n:]
	}
	if rootFields[2] != 1 || rootFields[22] != 1 || rootFields[24] != 1 {
		t.Fatalf("missing rewritten Wi-Fi/cell fields: %#v", rootFields)
	}
	if rootFields[3] != 0 || rootFields[4] != 0 || rootFields[33] != 0 {
		t.Fatalf("conflicting root fields were not removed: %#v", rootFields)
	}

	locationBytes, _ := rewriteLocation(nil, lat, lon)
	location := &pb.Location{}
	if err := proto.Unmarshal(locationBytes, location); err != nil {
		t.Fatalf("failed to decode rewritten location: %v", err)
	}
	if location.GetLatitude() != lat || location.GetLongitude() != lon {
		t.Fatalf("coordinates do not match: %d,%d", location.GetLatitude(), location.GetLongitude())
	}
	if location.GetHorizontalAccuracy() != 39 || location.GetVerticalAccuracy() != 1000 || location.GetAltitude() != 530 {
		t.Fatalf("accuracy or altitude fields do not match: %#v", location)
	}
	if location.GetMotionActivityType() != 63 || location.GetMotionActivityConfidence() != 467 {
		t.Fatalf("motion fields do not match: %#v", location)
	}
}

func TestCoordinateEncoding(t *testing.T) {
	lat := IntFromCoord(51.510420)
	lon := IntFromCoord(-3.218306)

	t.Logf("Lat 51.510420 -> %d", lat)
	t.Logf("Lon -3.218306 -> %d", lon)

	latZigzag := encodeZigzag(lat)
	lonZigzag := encodeZigzag(lon)

	t.Logf("Lat zigzag: %d (0x%x)", latZigzag, latZigzag)
	t.Logf("Lon zigzag: %d (0x%x)", lonZigzag, lonZigzag)

	latVarint := appendVarint(make([]byte, 0), latZigzag)
	lonVarint := appendVarint(make([]byte, 0), lonZigzag)

	t.Logf("Lat varint hex: %s", hex.EncodeToString(latVarint))
	t.Logf("Lon varint hex: %s", hex.EncodeToString(lonVarint))
}

func TestFieldPresence(t *testing.T) {
	wloc := &pb.AppleWLoc{
		WifiDevices: []*pb.WifiDevice{
			{
				Bssid: "aa:bb:cc:dd:ee:ff",
				Location: &pb.Location{
					Latitude:  p64(5151042000),
					Longitude: p64(-321830600),
				},
			},
		},
	}

	wloc.NumCellResults = nil
	wloc.NumWifiResults = nil
	wloc.DeviceType = nil

	protoBytes, _ := proto.Marshal(wloc)
	t.Logf("With nil fields: %d bytes", len(protoBytes))
	t.Logf("Hex: %x", protoBytes)

	wloc3 := &pb.AppleWLoc{
		WifiDevices: []*pb.WifiDevice{
			{
				Bssid: "aa:bb:cc:dd:ee:ff",
				Location: &pb.Location{
					Latitude:  p64(5151042000),
					Longitude: p64(-321830600),
				},
			},
		},
	}

	protoBytes3, _ := proto.Marshal(wloc3)
	t.Logf("Without setting fields: %d bytes", len(protoBytes3))
	t.Logf("Hex: %x", protoBytes3)
}
