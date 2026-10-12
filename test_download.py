import sys
import usb.core
import usb.util

# ---------------------------------------------------------
# CONSTANTS & CONFIGURATION
# ---------------------------------------------------------
ROLAND_VID = 0x0582
# Bus 001 Device 060: ID 0582:029b Roland Corp. SPD-SX PRO
# Bus 001 Device 061: ID 0582:0299 Roland Corp. SPD-SX PRO
SPDSX_PRO_PID = 0x299
VENDOR_INTERFACE = 4   # Interface handling proprietary App link

# Endpoint pairs discovered via Wireshark/usbmon
BULK_OUT_EP = 0x02  # Host sends request packets here
BULK_IN_EP = 0x82   # Host reads incoming file streams here
PACKET_SIZE = 512   # Maximum block size for High-Speed USB Bulk

# ---------------------------------------------------------
# 1. INITIALIZE HARDWARE INTERFACE
# ---------------------------------------------------------
dev = usb.core.find(idVendor=ROLAND_VID, idProduct=SPDSX_PRO_PID)

if dev is None:
    print("Error: SPD-SX PRO not found.")
    sys.exit(1)

# Reclaim interface control from Linux ALSA subsystem
if dev.is_kernel_driver_active(VENDOR_INTERFACE):
    try:
        dev.detach_kernel_driver(VENDOR_INTERFACE)
    except usb.core.USBError as e:
        print(f"Failed detaching kernel driver: {e}")
        sys.exit(1)

usb.util.claim_interface(dev, VENDOR_INTERFACE)
dev.set_configuration()

# ---------------------------------------------------------
# 2. RAW BULK DOWNLOAD PIPELINE
# ---------------------------------------------------------
def download_file_raw(output_filename):
    print(f"Initializing transfer to pull data into '{output_filename}'...")

    # --- CRITICAL REVERSE ENGINEERING GAP ---
    # The device sits completely silent until you ask for data. 
    # You must send the proprietary Roland "Fetch/Read Command" first.
    # This usually contains: Command Type (Read), Target Memory Slot / Wave ID, and Length.
    # ----------------------------------------
    # Example mock-up of a Roland Read Request (often structured like a MIDI RQ1 message):
    mock_read_request = bytes([0x41, 0x10, 0x00, 0x00, 0x11, 0x01, 0x00, 0x00]) 
    
    try:
        # Trigger the device to start outputting the target file bytes
        dev.write(BULK_OUT_EP, mock_read_request, timeout=2000)
    except usb.core.USBError as e:
        print(f"Failed to issue download request: {e}")
        return

    # Open target file locally to write incoming raw binary stream
    with open(output_filename, "wb") as f:
        bytes_received = 0
        
        while True:
            try:
                # Poll the bulk IN pipe for file segments
                # We read up to PACKET_SIZE bytes with a standard timeout
                chunk = dev.read(BULK_IN_EP, PACKET_SIZE, timeout=2000)
                
                if not chunk:
                    # An empty packet or 0-length buffer often signals End-Of-File (EOF)
                    print("\nReceived empty trailing packet. Ending transfer.")
                    break
                
                # --- CRITICAL REVERSE ENGINEERING GAP ---
                # Incoming raw data blocks will likely contain Roland headers or 
                # packet indexes. You must strip them out here before writing to disk.
                # e.g., cleaned_chunk = chunk[header_length : -checksum_length]
                # ----------------------------------------
                cleaned_chunk = chunk 
                
                f.write(cleaned_chunk)
                bytes_received += len(cleaned_chunk)
                
                print(f"Downloaded: {bytes_received} bytes", end="\r")
                
                # Logic break conditional: Some protocols don't send a 0-length final packet.
                # If your Wireshark traces show Roland expects the host to stop reading after 
                # a predefined file size is met, implement that break condition here.

            except usb.core.USBError as e:
                # If the device has no more data to give, it will time out
                if e.errno == 110 or "timeout" in str(e).lower():
                    print("\nPipe went quiet (Timeout). Assuming file transfer finished.")
                    break
                else:
                    print(f"\nError occurred during ingestion loop: {e}")
                    break

    print(f"Download routine completed. Total raw bytes saved: {bytes_received}")

# ---------------------------------------------------------
# EXECUTION
# ---------------------------------------------------------
if __name__ == "__main__":
    download_file_raw("downloaded_sample.wav")
    
    # Release interface cleanly on exit
    usb.util.release_interface(dev, VENDOR_INTERFACE)


