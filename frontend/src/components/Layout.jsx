import { Outlet } from "react-router-dom";
import Navbar from "./Navbar";
import Footer from "./Footer";
import ChatBubble from "./ChatBubble";
import { useAuth } from "../context/AuthContext";

function Layout() {
  const { isAuthenticated } = useAuth();

  return (
    <div className="app-shell">
      <Navbar />
      <main className="app-main">
        <Outlet />
      </main>
      <Footer />
      {isAuthenticated && <ChatBubble />}
    </div>
  );
}

export default Layout;
