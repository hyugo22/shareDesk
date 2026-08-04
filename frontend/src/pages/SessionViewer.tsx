import { useEffect, useRef, useState } from "react";
import { useLocation, useNavigate, useParams } from "react-router-dom";
import { apiJSON, getAccessToken, wsBaseURL } from "../api/client";
import { virtualKeyCode } from "../webrtc/keymap";

type SignalMessage = { type: string; session_id?: string; payload?: unknown };

export default function SessionViewer() {
  const { sessionId } = useParams<{ sessionId: string }>();
  const location = useLocation();
  const navigate = useNavigate();
  const agentName = (location.state as { agentName?: string })?.agentName ?? sessionId;

  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const [status, setStatus] = useState("Connexion en cours…");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!sessionId) return;
    let closed = false;
    const pc = new RTCPeerConnection();
    const ws = new WebSocket(`${wsBaseURL()}/ws?token=${getAccessToken()}&session_id=${sessionId}`);
    const control = pc.createDataChannel("control");

    function send(msg: SignalMessage) {
      if (ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify(msg));
    }

    ws.onopen = async () => {
      const offer = await pc.createOffer();
      await pc.setLocalDescription(offer);
      send({ type: "offer", session_id: sessionId, payload: { sdp: offer.sdp } });
    };

    ws.onmessage = async (evt) => {
      const msg: SignalMessage = JSON.parse(evt.data);
      if (msg.type === "answer") {
        const { sdp } = msg.payload as { sdp: string };
        await pc.setRemoteDescription({ type: "answer", sdp });
        setStatus("Connecté");
      } else if (msg.type === "ice-candidate") {
        try {
          await pc.addIceCandidate(msg.payload as RTCIceCandidateInit);
        } catch {
          // candidat déjà expiré / hors ordre : sans conséquence pour WebRTC
        }
      }
    };

    ws.onerror = () => setError("Erreur de connexion au serveur de signalisation");
    ws.onclose = () => {
      if (!closed) setStatus("Déconnecté");
    };

    pc.onicecandidate = (evt) => {
      if (evt.candidate) send({ type: "ice-candidate", session_id: sessionId, payload: evt.candidate.toJSON() });
    };

    pc.onconnectionstatechange = () => {
      if (pc.connectionState === "failed" || pc.connectionState === "disconnected") {
        setStatus("Connexion perdue");
      }
    };

    pc.ondatachannel = (evt) => {
      if (evt.channel.label !== "frames") return;
      evt.channel.binaryType = "arraybuffer";
      evt.channel.onmessage = async (msgEvt) => {
        const canvas = canvasRef.current;
        if (!canvas) return;
        const blob = new Blob([msgEvt.data], { type: "image/jpeg" });
        const bitmap = await createImageBitmap(blob);
        if (canvas.width !== bitmap.width) canvas.width = bitmap.width;
        if (canvas.height !== bitmap.height) canvas.height = bitmap.height;
        canvas.getContext("2d")?.drawImage(bitmap, 0, 0);
        bitmap.close();
      };
    };

    function normalizedCoords(e: React.MouseEvent | MouseEvent) {
      const canvas = canvasRef.current;
      if (!canvas) return { x: 0, y: 0 };
      const rect = canvas.getBoundingClientRect();
      const relX = Math.min(Math.max((e.clientX - rect.left) / rect.width, 0), 1);
      const relY = Math.min(Math.max((e.clientY - rect.top) / rect.height, 0), 1);
      return { x: Math.round(relX * 65535), y: Math.round(relY * 65535) };
    }

    function sendIfOpen(payload: unknown) {
      if (control.readyState === "open") control.send(JSON.stringify(payload));
    }

    const canvas = canvasRef.current;
    const buttonName = (b: number) => (b === 2 ? "right" : b === 1 ? "middle" : "left");

    const onMouseMove = (e: MouseEvent) => sendIfOpen({ type: "mousemove", ...normalizedCoords(e) });
    const onMouseDown = (e: MouseEvent) => sendIfOpen({ type: "mousedown", button: buttonName(e.button) });
    const onMouseUp = (e: MouseEvent) => sendIfOpen({ type: "mouseup", button: buttonName(e.button) });
    const onWheel = (e: WheelEvent) => {
      e.preventDefault();
      sendIfOpen({ type: "wheel", dx: -Math.round(e.deltaX), dy: -Math.round(e.deltaY) });
    };
    const onContextMenu = (e: MouseEvent) => e.preventDefault();
    const onKeyDown = (e: KeyboardEvent) => {
      e.preventDefault();
      const vk = virtualKeyCode(e.code);
      if (vk !== null) sendIfOpen({ type: "keydown", key: vk });
    };
    const onKeyUp = (e: KeyboardEvent) => {
      e.preventDefault();
      const vk = virtualKeyCode(e.code);
      if (vk !== null) sendIfOpen({ type: "keyup", key: vk });
    };

    canvas?.addEventListener("mousemove", onMouseMove);
    canvas?.addEventListener("mousedown", onMouseDown);
    canvas?.addEventListener("mouseup", onMouseUp);
    canvas?.addEventListener("wheel", onWheel, { passive: false });
    canvas?.addEventListener("contextmenu", onContextMenu);
    canvas?.setAttribute("tabindex", "0");
    canvas?.addEventListener("keydown", onKeyDown);
    canvas?.addEventListener("keyup", onKeyUp);

    return () => {
      closed = true;
      canvas?.removeEventListener("mousemove", onMouseMove);
      canvas?.removeEventListener("mousedown", onMouseDown);
      canvas?.removeEventListener("mouseup", onMouseUp);
      canvas?.removeEventListener("wheel", onWheel);
      canvas?.removeEventListener("contextmenu", onContextMenu);
      canvas?.removeEventListener("keydown", onKeyDown);
      canvas?.removeEventListener("keyup", onKeyUp);
      pc.close();
      ws.close();
      apiJSON(`/sessions/${sessionId}/end`, { method: "POST" }).catch(() => {});
    };
  }, [sessionId]);

  return (
    <div className="session-viewer">
      <div className="session-toolbar">
        <button onClick={() => navigate("/agents")}>← Retour</button>
        <span>{agentName}</span>
        <span className="badge">{status}</span>
      </div>
      {error && <p className="error">{error}</p>}
      <canvas ref={canvasRef} className="session-canvas" />
    </div>
  );
}
