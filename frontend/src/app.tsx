import {
  App as AntdApp,
  Alert,
  Avatar,
  Badge,
  Button,
  Card,
  Col,
  ConfigProvider,
  Descriptions,
  Divider,
  Drawer,
  Empty,
  Form,
  Input,
  Layout,
  List,
  Menu,
  Popconfirm,
  Row,
  Space,
  Spin,
  Statistic,
  Switch,
  Table,
  Tag,
  Typography,
  message,
} from "antd";
import {
  CloudServerOutlined,
  DashboardOutlined,
  LinkOutlined,
  LogoutOutlined,
  NotificationOutlined,
  ReloadOutlined,
  SafetyCertificateOutlined,
  SettingOutlined,
  TagsOutlined,
  UserOutlined,
} from "@ant-design/icons";
import {
  createContext,
  startTransition,
  useContext,
  useEffect,
  useEffectEvent,
  useState,
  type ReactNode,
} from "react";
import {
  BrowserRouter,
  Navigate,
  Outlet,
  Route,
  Routes,
  useLocation,
  useNavigate,
} from "react-router-dom";

import { requestJSON, readStoredAuth, storeAuth, unwrapData } from "./api";
import type {
  ApiEnvelope,
  DashboardStats,
  Notice,
  Plan,
  ServerNode,
  SessionCheck,
  SettingsRecord,
  SubscribePayload,
  SubscriptionLink,
  UserInfo,
  UserRow,
} from "./types";
import {
  formatBytes,
  formatDateTime,
  formatPrice,
  formatRelativeStamp,
  initialsFromEmail,
  primarySubscription,
  readBootstrap,
  trafficSummary,
} from "./utils";

const bootstrap = readBootstrap();

const { Header, Content, Sider } = Layout;
const { Paragraph, Text, Title } = Typography;

interface SessionState {
  ready: boolean;
  checking: boolean;
  isAuthenticated: boolean;
  isAdmin: boolean;
}

interface SessionContextValue {
  authData: string;
  session: SessionState;
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string) => Promise<void>;
  logout: () => void;
  refreshSession: () => Promise<void>;
}

const SessionContext = createContext<SessionContextValue | null>(null);

interface UserDashboardState {
  loading: boolean;
  info: UserInfo | null;
  subscribe: SubscribePayload | null;
  invites: number;
}

interface UserDashboardContextValue extends UserDashboardState {
  refreshOverview: () => Promise<void>;
}

const UserDashboardContext = createContext<UserDashboardContextValue | null>(null);

export default function App() {
  return (
    <ConfigProvider
      theme={{
        token: {
          colorPrimary: "#0665d0",
          borderRadius: 14,
          colorBgLayout: "#f4f7fb",
          colorTextBase: "#102038",
          colorTextSecondary: "#5b6882",
        },
      }}
    >
      <AntdApp>
        <SessionProvider>{bootstrap.page === "admin" ? <AdminRoot /> : <UserRoot />}</SessionProvider>
      </AntdApp>
    </ConfigProvider>
  );
}

function SessionProvider({ children }: { children: ReactNode }) {
  const [authData, setAuthData] = useState(readStoredAuth());
  const [session, setSession] = useState<SessionState>({
    ready: false,
    checking: false,
    isAuthenticated: false,
    isAdmin: false,
  });

  const applyAuth = (value: string) => {
    storeAuth(value);
    setAuthData(value);
  };

  const verifySession = useEffectEvent(async (currentAuth: string) => {
    if (!currentAuth) {
      startTransition(() => {
        setSession({
          ready: true,
          checking: false,
          isAuthenticated: false,
          isAdmin: false,
        });
      });
      return;
    }

    startTransition(() => {
      setSession((prev) => ({ ...prev, checking: true }));
    });

    try {
      const payload = await requestJSON<ApiEnvelope<SessionCheck>>(`${bootstrap.apiBase}/user/checkLogin`, {
        authData: currentAuth,
        silent: true,
      });
      const data = unwrapData(payload);
      if (!data.is_login) {
        applyAuth("");
      }
      startTransition(() => {
        setSession({
          ready: true,
          checking: false,
          isAuthenticated: Boolean(data.is_login),
          isAdmin: Boolean(data.is_admin),
        });
      });
    } catch (_) {
      applyAuth("");
      startTransition(() => {
        setSession({
          ready: true,
          checking: false,
          isAuthenticated: false,
          isAdmin: false,
        });
      });
    }
  });

  useEffect(() => {
    void verifySession(authData);
  }, [authData]);

  const login = async (email: string, password: string) => {
    const payload = await requestJSON<ApiEnvelope<{ auth_data: string }>>(
      `${bootstrap.apiBase}/passport/auth/login`,
      {
        method: "POST",
        body: { email, password },
      },
    );
    applyAuth(unwrapData(payload).auth_data);
    message.success("登录成功");
  };

  const register = async (email: string, password: string) => {
    const payload = await requestJSON<ApiEnvelope<{ auth_data: string }>>(
      `${bootstrap.apiBase}/passport/auth/register`,
      {
        method: "POST",
        body: { email, password },
      },
    );
    applyAuth(unwrapData(payload).auth_data);
    message.success("注册成功");
  };

  const logout = () => {
    applyAuth("");
    startTransition(() => {
      setSession({
        ready: true,
        checking: false,
        isAuthenticated: false,
        isAdmin: false,
      });
    });
    message.success("已退出登录");
  };

  const value: SessionContextValue = {
    authData,
    session,
    login,
    register,
    logout,
    refreshSession: async () => verifySession(readStoredAuth()),
  };

  return <SessionContext value={value}>{children}</SessionContext>;
}

function useSession() {
  const context = useContext(SessionContext);
  if (!context) {
    throw new Error("session context is missing");
  }
  return context;
}

function UserDashboardProvider({ children }: { children: ReactNode }) {
  const { authData } = useSession();
  const [state, setState] = useState<UserDashboardState>({
    loading: true,
    info: null,
    subscribe: null,
    invites: 0,
  });

  const loadOverview = useEffectEvent(async () => {
    if (!authData) {
      return;
    }
    setState((prev) => ({ ...prev, loading: true }));
    try {
      const [infoPayload, subscribePayload, statPayload] = await Promise.all([
        requestJSON<ApiEnvelope<UserInfo>>(`${bootstrap.apiBase}/user/info`, { authData }),
        requestJSON<ApiEnvelope<SubscribePayload>>(`${bootstrap.apiBase}/user/getSubscribe`, { authData }),
        requestJSON<ApiEnvelope<number[]>>(`${bootstrap.apiBase}/user/getStat`, { authData }),
      ]);
      startTransition(() => {
        setState({
          loading: false,
          info: unwrapData(infoPayload),
          subscribe: unwrapData(subscribePayload),
          invites: unwrapData(statPayload)?.[2] || 0,
        });
      });
    } catch (_) {
      startTransition(() => {
        setState((prev) => ({ ...prev, loading: false }));
      });
    }
  });

  useEffect(() => {
    void loadOverview();
  }, [authData]);

  return (
    <UserDashboardContext
      value={{
        ...state,
        refreshOverview: async () => loadOverview(),
      }}
    >
      {children}
    </UserDashboardContext>
  );
}

function useUserDashboard() {
  const context = useContext(UserDashboardContext);
  if (!context) {
    throw new Error("user dashboard context is missing");
  }
  return context;
}

function UserRoot() {
  const { session } = useSession();

  if (!session.ready) {
    return <FullscreenLoading title="正在加载用户面板" />;
  }

  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<PublicLanding />} />
        <Route
          path="/dashboard"
          element={
            <RequireUser>
              <UserDashboardProvider>
                <UserLayout />
              </UserDashboardProvider>
            </RequireUser>
          }
        >
          <Route index element={<UserOverviewPage />} />
          <Route path="servers" element={<UserServersPage />} />
          <Route path="plans" element={<UserPlansPage />} />
          <Route path="settings" element={<UserSettingsPage />} />
        </Route>
        <Route path="*" element={<UserFallbackRedirect />} />
      </Routes>
    </BrowserRouter>
  );
}

function AdminRoot() {
  const { session } = useSession();

  if (!session.ready) {
    return <FullscreenLoading title="正在加载管理面板" />;
  }

  return (
    <BrowserRouter basename={`/${bootstrap.adminPath}`}>
      <Routes>
        <Route path="/login" element={<AdminLoginPage />} />
        <Route
          path="/"
          element={
            <RequireAdmin>
              <AdminLayout />
            </RequireAdmin>
          }
        >
          <Route index element={<Navigate to="/overview" replace />} />
          <Route path="overview" element={<AdminOverviewPage />} />
          <Route path="subscription-links" element={<Navigate to="/users" replace />} />
          <Route path="users" element={<AdminUsersPage />} />
          <Route path="plans" element={<AdminPlansPage />} />
          <Route path="servers" element={<AdminServersPage />} />
          <Route path="notices" element={<AdminNoticesPage />} />
          <Route path="settings" element={<AdminSettingsPage />} />
        </Route>
        <Route path="*" element={<AdminFallbackRedirect />} />
      </Routes>
    </BrowserRouter>
  );
}

function RequireUser({ children }: { children: ReactNode }) {
  const { session } = useSession();
  if (!session.ready) {
    return <FullscreenLoading title="正在校验登录状态" />;
  }
  if (!session.isAuthenticated) {
    return <Navigate to="/" replace />;
  }
  return <>{children}</>;
}

function RequireAdmin({ children }: { children: ReactNode }) {
  const { session } = useSession();
  if (!session.ready) {
    return <FullscreenLoading title="正在校验管理员权限" />;
  }
  if (!session.isAuthenticated || !session.isAdmin) {
    return <Navigate to="/login" replace />;
  }
  return <>{children}</>;
}

function UserFallbackRedirect() {
  const { session } = useSession();
  return <Navigate to={session.isAuthenticated ? "/dashboard" : "/"} replace />;
}

function AdminFallbackRedirect() {
  const { session } = useSession();
  return <Navigate to={session.isAuthenticated && session.isAdmin ? "/overview" : "/login"} replace />;
}

function PublicLanding() {
  const { session, login, register } = useSession();
  const navigate = useNavigate();
  const [plans, setPlans] = useState<Plan[]>([]);
  const [loading, setLoading] = useState(true);
  const [loginLoading, setLoginLoading] = useState(false);
  const [registerLoading, setRegisterLoading] = useState(false);

  const [loginForm] = Form.useForm<{ email: string; password: string }>();
  const [registerForm] = Form.useForm<{ email: string; password: string }>();

  useEffect(() => {
    if (session.isAuthenticated) {
      navigate("/dashboard", { replace: true });
    }
  }, [navigate, session.isAuthenticated]);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      setLoading(true);
      try {
        const payload = await requestJSON<ApiEnvelope<Plan[]>>(`${bootstrap.apiBase}/guest/plan/fetch`, {
          silent: true,
        });
        if (!cancelled) {
          setPlans(unwrapData(payload));
        }
      } catch (_) {
        if (!cancelled) {
          setPlans([]);
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  const handleLogin = async (values: { email: string; password: string }) => {
    setLoginLoading(true);
    try {
      await login(values.email, values.password);
      navigate("/dashboard", { replace: true });
    } finally {
      setLoginLoading(false);
    }
  };

  const handleRegister = async (values: { email: string; password: string }) => {
    setRegisterLoading(true);
    try {
      await register(values.email, values.password);
      navigate("/dashboard", { replace: true });
    } finally {
      setRegisterLoading(false);
    }
  };

  return (
    <div className="landing-shell">
      <section className="landing-grid">
        <div className="landing-hero panel-card panel-hero">
          <div className="brand-row">
            <div className="brand-mark">A</div>
            <div>
              <Text className="brand-name">{bootstrap.title}</Text>
              <Text className="brand-subtitle">Go · SQLite · Redis · Xboard-style Panel</Text>
            </div>
          </div>
          <div className="hero-tags">
            <Tag>多订阅后缀</Tag>
            <Tag>Clash / V2 / Shadowrocket</Tag>
            <Tag>UniProxy 节点对接</Tag>
          </div>
          <Title level={1}>订阅管理面板</Title>
          <Paragraph>
            按照面板的结构重写为现代前端应用。用户端与管理员端都保留真实布局、侧边导航、数据表格和表单操作，而不是静态模板。
          </Paragraph>
          <div className="hero-feature-grid">
            <Card variant="borderless" className="hero-feature-card">
              <Text strong>用户面板</Text>
              <Text>订阅、节点、套餐、流量、到期时间、账号设置</Text>
            </Card>
            <Card variant="borderless" className="hero-feature-card">
              <Text strong>管理员面板</Text>
              <Text>用户、套餐、节点、公告、站点配置、订阅后缀</Text>
            </Card>
            <Card variant="borderless" className="hero-feature-card">
              <Text strong>兼容 API</Text>
              <Text>/api/v1/passport /user /client /server/UniProxy</Text>
            </Card>
            <Card variant="borderless" className="hero-feature-card">
              <Text strong>基础范围</Text>
              <Text>不包含支付、订单、工单、优惠券、知识库和路由管理</Text>
            </Card>
          </div>
        </div>

        <Card className="panel-card auth-panel-card">
          <div className="auth-card-head">
            <Title level={3}>登录或注册</Title>
            <Text type="secondary">默认管理员: admin@example.com / admin123456</Text>
          </div>
          <Row gutter={[16, 16]}>
            <Col span={24}>
              <Card className="mini-form-card" title="账户登录">
                <Form layout="vertical" form={loginForm} onFinish={handleLogin}>
                  <Form.Item name="email" label="邮箱" rules={[{ required: true }]}>
                    <Input placeholder="demo@example.com" />
                  </Form.Item>
                  <Form.Item name="password" label="密码" rules={[{ required: true }]}>
                    <Input.Password placeholder="请输入密码" />
                  </Form.Item>
                  <Button type="primary" htmlType="submit" block loading={loginLoading}>
                    登录
                  </Button>
                </Form>
              </Card>
            </Col>
            <Col span={24}>
              <Card className="mini-form-card" title="创建账户">
                <Form layout="vertical" form={registerForm} onFinish={handleRegister}>
                  <Form.Item name="email" label="邮箱" rules={[{ required: true }]}>
                    <Input placeholder="you@example.com" />
                  </Form.Item>
                  <Form.Item name="password" label="密码" rules={[{ required: true }]}>
                    <Input.Password placeholder="至少 8 位" />
                  </Form.Item>
                  <Button htmlType="submit" block loading={registerLoading}>
                    注册并登录
                  </Button>
                </Form>
              </Card>
            </Col>
          </Row>
        </Card>
      </section>

      <Card className="panel-card">
        <Space direction="vertical" size={18} style={{ width: "100%" }}>
          <div className="section-head">
            <div>
              <Title level={3}>套餐预览</Title>
              <Text type="secondary">直接读取兼容的 `/api/v1/guest/plan/fetch`。</Text>
            </div>
          </div>
          {loading ? (
            <div className="page-spin">
              <Spin />
            </div>
          ) : plans.length === 0 ? (
            <Empty description="暂无套餐" />
          ) : (
            <Row gutter={[16, 16]}>
              {plans.map((plan) => (
                <Col xs={24} md={12} xl={6} key={plan.id}>
                  <Card className="plan-card" hoverable>
                    <Space direction="vertical" size={10} style={{ width: "100%" }}>
                      <Tag color="blue">PLAN {plan.id}</Tag>
                      <Title level={4}>{plan.name}</Title>
                      <Text className="plan-price">{formatPrice(plan.price)}</Text>
                      <Text type="secondary">{plan.content || "暂无描述"}</Text>
                      <Space wrap>
                        <Tag>{formatBytes(plan.transfer_enable)}</Tag>
                        <Tag>{plan.speed_limit} Mbps</Tag>
                      </Space>
                    </Space>
                  </Card>
                </Col>
              ))}
            </Row>
          )}
        </Space>
      </Card>
    </div>
  );
}

function UserLayout() {
  const { logout, session } = useSession();
  const { info, subscribe, loading, refreshOverview, invites } = useUserDashboard();
  const navigate = useNavigate();
  const location = useLocation();
  const selectedKey = location.pathname.startsWith("/dashboard/servers")
    ? "/dashboard/servers"
    : location.pathname.startsWith("/dashboard/plans")
      ? "/dashboard/plans"
      : location.pathname.startsWith("/dashboard/settings")
        ? "/dashboard/settings"
        : "/dashboard";

  return (
    <Layout className="panel-shell">
      <Sider breakpoint="lg" collapsedWidth="0" className="panel-sider">
        <div className="panel-brand">
          <div className="brand-mark small">A</div>
          <div>
            <div className="sider-title">{bootstrap.title}</div>
            <div className="sider-subtitle">Subscriber Console</div>
          </div>
        </div>
        <div className="sider-profile">
          <Avatar
            size={46}
            src={info?.avatar_url || undefined}
            icon={!info?.avatar_url ? <UserOutlined /> : undefined}
          >
            {initialsFromEmail(info?.email || "")}
          </Avatar>
          <div>
            <Text strong>{info?.email || "未登录"}</Text>
            <Text type="secondary">{subscribe?.plan?.name || "未分配套餐"}</Text>
          </div>
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[selectedKey]}
          onClick={({ key }) => navigate(key)}
          items={[
            { key: "/dashboard", icon: <DashboardOutlined />, label: "概览" },
            { key: "/dashboard/servers", icon: <CloudServerOutlined />, label: "节点" },
            { key: "/dashboard/plans", icon: <TagsOutlined />, label: "套餐" },
            { key: "/dashboard/settings", icon: <SettingOutlined />, label: "设置" },
          ]}
        />
        <div className="sider-footer">
          <Button block icon={<ReloadOutlined />} onClick={() => void refreshOverview()} loading={loading}>
            刷新概览
          </Button>
          {session.isAdmin && (
            <Button block onClick={() => navigate(`/${bootstrap.adminPath}`)}>
              管理员入口
            </Button>
          )}
          <Button block icon={<LogoutOutlined />} onClick={logout}>
            退出登录
          </Button>
        </div>
      </Sider>
      <Layout>
        <Header className="panel-header">
          <div>
            <Text className="header-kicker">User Dashboard</Text>
            <Title level={2}>用户面板</Title>
          </div>
          <Space wrap>
            <Tag color="blue">{subscribe?.plan?.name || "未分配套餐"}</Tag>
            <Tag>{`邀请用户 ${invites}`}</Tag>
          </Space>
        </Header>
        <Content className="panel-content">
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}

function UserOverviewPage() {
  const { authData } = useSession();
  const { info, subscribe, invites, refreshOverview, loading } = useUserDashboard();
  const [notices, setNotices] = useState<Notice[]>([]);
  const [noticesLoading, setNoticesLoading] = useState(true);
  const [resetLoading, setResetLoading] = useState(false);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      setNoticesLoading(true);
      try {
        const payload = await requestJSON<{ data: Notice[] }>(`${bootstrap.apiBase}/user/notice/fetch`, {
          authData,
        });
        if (!cancelled) {
          setNotices(payload.data || []);
        }
      } catch (_) {
        if (!cancelled) {
          setNotices([]);
        }
      } finally {
        if (!cancelled) {
          setNoticesLoading(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [authData]);

  const handleReset = async () => {
    setResetLoading(true);
    try {
      await requestJSON<ApiEnvelope<SubscriptionLink[]>>(`${bootstrap.apiBase}/user/resetSecurity`, {
        authData,
      });
      message.success("订阅信息已重置");
      await refreshOverview();
    } finally {
      setResetLoading(false);
    }
  };

  const currentPrimary = primarySubscription(subscribe?.subscribe_urls || []);

  return (
    <Space direction="vertical" size={18} style={{ width: "100%" }}>
      <Card className="panel-card banner-card">
        <div className="banner-grid">
          <div>
            <Space wrap>
              <Tag color="blue">Active</Tag>
              <Tag>{`到期 ${formatDateTime(subscribe?.expired_at || 0)}`}</Tag>
            </Space>
            <Title level={3}>{`欢迎回来，${(info?.email || "用户").split("@")[0]}`}</Title>
            <Paragraph>
              当前面板展示流量、套餐、多个订阅地址和节点信息，整体交互和结构按面板方式重做，不再是静态模板。
            </Paragraph>
          </div>
          <div className="banner-side">
            <Text type="secondary">主订阅后缀</Text>
            <Title level={4}>{currentPrimary?.suffix || "默认"}</Title>
          </div>
        </div>
      </Card>

      <Row gutter={[16, 16]}>
        <Col xs={24} md={12} xl={6}>
          <Card className="metric-card">
            <Statistic title="剩余 / 总流量" value={trafficSummary(subscribe?.u || 0, subscribe?.d || 0, subscribe?.transfer_enable || 0)} />
          </Card>
        </Col>
        <Col xs={24} md={12} xl={6}>
          <Card className="metric-card">
            <Statistic title="套餐到期" value={formatDateTime(subscribe?.expired_at || 0)} />
          </Card>
        </Col>
        <Col xs={24} md={12} xl={6}>
          <Card className="metric-card">
            <Statistic title="本月重置日" value={`${subscribe?.reset_day || "-"} 日`} />
          </Card>
        </Col>
        <Col xs={24} md={12} xl={6}>
          <Card className="metric-card">
            <Statistic title="推广用户" value={invites} />
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]}>
        <Col xs={24} xl={15}>
          <Card
            className="panel-card"
            title="订阅地址"
            extra={
              <Space>
                <Button icon={<ReloadOutlined />} onClick={() => void refreshOverview()} loading={loading}>
                  刷新
                </Button>
                <Button type="primary" icon={<SafetyCertificateOutlined />} onClick={() => void handleReset()} loading={resetLoading}>
                  重置订阅
                </Button>
              </Space>
            }
          >
            <Space direction="vertical" size={14} style={{ width: "100%" }}>
              {(subscribe?.subscribe_urls || []).map((item) => (
                <SubscriptionCard key={item.id} item={item} />
              ))}
              {!subscribe?.subscribe_urls?.length && <Empty description="暂无订阅地址" />}
            </Space>
          </Card>
        </Col>
        <Col xs={24} xl={9}>
          <Card className="panel-card" title="账号信息">
            <Descriptions column={1} size="small">
              <Descriptions.Item label="邮箱">{info?.email || "-"}</Descriptions.Item>
              <Descriptions.Item label="当前套餐">{subscribe?.plan?.name || "未分配套餐"}</Descriptions.Item>
              <Descriptions.Item label="UUID">{info?.uuid || "-"}</Descriptions.Item>
              <Descriptions.Item label="到期时间">{formatDateTime(subscribe?.expired_at || 0)}</Descriptions.Item>
              <Descriptions.Item label="创建时间">{formatDateTime(info?.created_at || 0)}</Descriptions.Item>
            </Descriptions>
          </Card>
          <Card className="panel-card" title="最新公告" styles={{ body: { paddingTop: 12 } }}>
            {noticesLoading ? (
              <div className="page-spin">
                <Spin />
              </div>
            ) : notices.length === 0 ? (
              <Empty description="暂无公告" />
            ) : (
              <List
                dataSource={notices}
                renderItem={(notice) => (
                  <List.Item>
                    <List.Item.Meta
                      title={notice.title}
                      description={
                        <Space direction="vertical" size={4}>
                          <Text type="secondary">{notice.content}</Text>
                          <Text type="secondary">{formatDateTime(notice.created_at)}</Text>
                        </Space>
                      }
                    />
                  </List.Item>
                )}
              />
            )}
          </Card>
        </Col>
      </Row>
    </Space>
  );
}

function UserServersPage() {
  const { authData } = useSession();
  const [loading, setLoading] = useState(true);
  const [servers, setServers] = useState<ServerNode[]>([]);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      setLoading(true);
      try {
        const payload = await requestJSON<{ data: ServerNode[] }>(`${bootstrap.apiBase}/user/server/fetch`, {
          authData,
        });
        if (!cancelled) {
          setServers(payload.data || []);
        }
      } catch (_) {
        if (!cancelled) {
          setServers([]);
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [authData]);

  return (
    <Card className="panel-card" title="节点列表">
      <Table
        rowKey="id"
        loading={loading}
        dataSource={servers}
        pagination={false}
        scroll={{ x: 920 }}
        columns={[
          {
            title: "节点",
            dataIndex: "name",
            render: (_, record: ServerNode) => (
              <Space direction="vertical" size={2}>
                <Text strong>{record.name}</Text>
                <Text type="secondary">{`${record.type} · 版本 ${record.version}`}</Text>
              </Space>
            ),
          },
          {
            title: "状态",
            dataIndex: "is_online",
            render: (value: boolean) => (
              <Badge status={value ? "success" : "error"} text={value ? "在线" : "离线"} />
            ),
          },
          {
            title: "倍率",
            dataIndex: "rate",
          },
          {
            title: "标签",
            dataIndex: "tags",
            render: (tags: string[]) => (
              <Space wrap>{(tags || []).length ? tags.map((item) => <Tag key={item}>{item}</Tag>) : <Text type="secondary">暂无标签</Text>}</Space>
            ),
          },
          {
            title: "最近检查",
            dataIndex: "last_check_at",
            render: (value: number) => formatDateTime(value),
          },
        ]}
      />
    </Card>
  );
}

function UserPlansPage() {
  const { subscribe } = useUserDashboard();
  const [plans, setPlans] = useState<Plan[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    void (async () => {
      setLoading(true);
      try {
        const payload = await requestJSON<ApiEnvelope<Plan[]>>(`${bootstrap.apiBase}/guest/plan/fetch`, {
          silent: true,
        });
        if (!cancelled) {
          setPlans(unwrapData(payload));
        }
      } catch (_) {
        if (!cancelled) {
          setPlans([]);
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  return (
    <Card className="panel-card" title="可用套餐">
      {loading ? (
        <div className="page-spin">
          <Spin />
        </div>
      ) : plans.length === 0 ? (
        <Empty description="暂无套餐" />
      ) : (
        <Row gutter={[16, 16]}>
          {plans.map((plan) => {
            const active = subscribe?.plan_id === plan.id;
            return (
              <Col xs={24} md={12} xl={8} key={plan.id}>
                <Card className={`plan-card ${active ? "is-active" : ""}`} hoverable>
                  <Space direction="vertical" size={10} style={{ width: "100%" }}>
                    <Space wrap>
                      <Tag color="blue">PLAN {plan.id}</Tag>
                      {active && <Tag color="green">当前套餐</Tag>}
                    </Space>
                    <Title level={4}>{plan.name}</Title>
                    <Text className="plan-price">{formatPrice(plan.price)}</Text>
                    <Paragraph type="secondary">{plan.content || "暂无描述"}</Paragraph>
                    <Space wrap>
                      <Tag>{formatBytes(plan.transfer_enable)}</Tag>
                      <Tag>{plan.speed_limit} Mbps</Tag>
                    </Space>
                  </Space>
                </Card>
              </Col>
            );
          })}
        </Row>
      )}
    </Card>
  );
}

function UserSettingsPage() {
  const { authData } = useSession();
  const { info, refreshOverview } = useUserDashboard();
  const [passwordForm] = Form.useForm<{ old_password: string; new_password: string }>();
  const [preferenceForm] = Form.useForm<{ remind_expire: boolean; remind_traffic: boolean }>();
  const [savingPassword, setSavingPassword] = useState(false);
  const [savingPreferences, setSavingPreferences] = useState(false);

  useEffect(() => {
    preferenceForm.setFieldsValue({
      remind_expire: Boolean(info?.remind_expire),
      remind_traffic: Boolean(info?.remind_traffic),
    });
  }, [info, preferenceForm]);

  const savePassword = async (values: { old_password: string; new_password: string }) => {
    setSavingPassword(true);
    try {
      await requestJSON<ApiEnvelope<boolean>>(`${bootstrap.apiBase}/user/changePassword`, {
        method: "POST",
        authData,
        body: values,
      });
      passwordForm.resetFields();
      message.success("密码已更新");
    } finally {
      setSavingPassword(false);
    }
  };

  const savePreferences = async (values: { remind_expire: boolean; remind_traffic: boolean }) => {
    setSavingPreferences(true);
    try {
      await requestJSON<ApiEnvelope<boolean>>(`${bootstrap.apiBase}/user/update`, {
        method: "POST",
        authData,
        body: values,
      });
      message.success("提醒设置已保存");
      await refreshOverview();
    } finally {
      setSavingPreferences(false);
    }
  };

  return (
    <Row gutter={[16, 16]}>
      <Col xs={24} xl={12}>
        <Card className="panel-card" title="修改密码">
          <Form layout="vertical" form={passwordForm} onFinish={savePassword}>
            <Form.Item name="old_password" label="旧密码" rules={[{ required: true }]}>
              <Input.Password />
            </Form.Item>
            <Form.Item name="new_password" label="新密码" rules={[{ required: true }]}>
              <Input.Password />
            </Form.Item>
            <Button type="primary" htmlType="submit" loading={savingPassword}>
              保存密码
            </Button>
          </Form>
        </Card>
      </Col>
      <Col xs={24} xl={12}>
        <Card className="panel-card" title="提醒设置">
          <Form layout="vertical" form={preferenceForm} onFinish={savePreferences}>
            <Form.Item name="remind_expire" valuePropName="checked">
              <Switch checkedChildren="到期提醒" unCheckedChildren="到期提醒" />
            </Form.Item>
            <Form.Item name="remind_traffic" valuePropName="checked">
              <Switch checkedChildren="流量提醒" unCheckedChildren="流量提醒" />
            </Form.Item>
            <Button type="primary" htmlType="submit" loading={savingPreferences}>
              保存设置
            </Button>
          </Form>
        </Card>
      </Col>
    </Row>
  );
}

function AdminLoginPage() {
  const { session, login, refreshSession } = useSession();
  const navigate = useNavigate();
  const [loading, setLoading] = useState(false);
  const [form] = Form.useForm<{ email: string; password: string }>();

  useEffect(() => {
    if (session.isAuthenticated && session.isAdmin) {
      navigate("/overview", { replace: true });
    }
  }, [navigate, session.isAdmin, session.isAuthenticated]);

  const handleSubmit = async (values: { email: string; password: string }) => {
    setLoading(true);
    try {
      await login(values.email, values.password);
      await refreshSession();
      navigate("/overview", { replace: true });
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="landing-shell admin-login-shell">
      <section className="landing-grid">
        <div className="landing-hero panel-card panel-hero">
          <div className="brand-row">
            <div className="brand-mark">A</div>
            <div>
              <Text className="brand-name">{bootstrap.title}</Text>
              <Text className="brand-subtitle">Admin Control Plane</Text>
            </div>
          </div>
          <div className="hero-tags">
            <Tag>管理员登录</Tag>
            <Tag>用户 / 套餐 / 节点 / 公告 / 配置</Tag>
            <Tag>多订阅后缀管理</Tag>
          </div>
          <Title level={1}>基础管理面板</Title>
          <Paragraph>
            用现代前端重写后的管理端，保留面板结构和操作逻辑。当前范围只覆盖基础订阅运营，不重新引入支付、订单、工单等复杂模块。
          </Paragraph>
        </div>

        <Card className="panel-card auth-panel-card">
          <div className="auth-card-head">
            <Title level={3}>管理员登录</Title>
            <Text type="secondary">{`当前安全路径 /${bootstrap.adminPath}`}</Text>
          </div>
          <Form layout="vertical" form={form} onFinish={handleSubmit}>
            <Form.Item name="email" label="邮箱" rules={[{ required: true }]}>
              <Input placeholder="admin@example.com" />
            </Form.Item>
            <Form.Item name="password" label="密码" rules={[{ required: true }]}>
              <Input.Password placeholder="请输入密码" />
            </Form.Item>
            <Button type="primary" htmlType="submit" block loading={loading}>
              进入管理台
            </Button>
          </Form>
        </Card>
      </section>
    </div>
  );
}

function AdminLayout() {
  const { logout } = useSession();
  const navigate = useNavigate();
  const location = useLocation();
  const selectedKey = location.pathname.startsWith("/plans")
    ? "/plans"
    : location.pathname.startsWith("/servers")
      ? "/servers"
      : location.pathname.startsWith("/notices")
        ? "/notices"
        : location.pathname.startsWith("/settings")
          ? "/settings"
          : location.pathname.startsWith("/users")
            ? "/users"
            : "/overview";

  return (
    <Layout className="panel-shell">
      <Sider breakpoint="lg" collapsedWidth="0" className="panel-sider">
        <div className="panel-brand">
          <div className="brand-mark small">A</div>
          <div>
            <div className="sider-title">{bootstrap.title}</div>
            <div className="sider-subtitle">Admin Console</div>
          </div>
        </div>
        <div className="sider-profile compact">
          <div>
            <Text strong>Secure Path</Text>
            <Text type="secondary">{`/${bootstrap.adminPath}`}</Text>
          </div>
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[selectedKey]}
          onClick={({ key }) => navigate(key)}
          items={[
            { key: "/overview", icon: <DashboardOutlined />, label: "概览" },
            { key: "/users", icon: <UserOutlined />, label: "用户" },
            { key: "/plans", icon: <TagsOutlined />, label: "套餐" },
            { key: "/servers", icon: <CloudServerOutlined />, label: "节点" },
            { key: "/notices", icon: <NotificationOutlined />, label: "公告" },
            { key: "/settings", icon: <SettingOutlined />, label: "系统" },
          ]}
        />
        <div className="sider-footer">
          <Button block onClick={() => window.location.assign("/")}>
            前台首页
          </Button>
          <Button block icon={<LogoutOutlined />} onClick={logout}>
            退出登录
          </Button>
        </div>
      </Sider>
      <Layout>
        <Header className="panel-header">
          <div>
            <Text className="header-kicker">Admin Dashboard</Text>
            <Title level={2}>管理面板</Title>
          </div>
        </Header>
        <Content className="panel-content">
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
}

function AdminOverviewPage() {
  const { authData } = useSession();
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [loading, setLoading] = useState(true);

  const loadStats = useEffectEvent(async () => {
    setLoading(true);
    try {
      const payload = await requestJSON<ApiEnvelope<DashboardStats>>(
        `${bootstrap.apiBase}/${bootstrap.adminPath}/stat/getStat`,
        { authData },
      );
      setStats(unwrapData(payload));
    } finally {
      setLoading(false);
    }
  });

  useEffect(() => {
    void loadStats();
  }, []);

  return (
    <Space direction="vertical" size={18} style={{ width: "100%" }}>
      <Card className="panel-card">
        <div className="banner-grid">
          <div>
            <Space wrap>
              <Tag color="blue">Core Scope</Tag>
              <Tag>SQLite + Redis</Tag>
            </Space>
            <Title level={3}>基础运营范围</Title>
            <Paragraph>
              当前管理端只保留你明确要求的基础模块: 管理员登录、节点对接、流量统计、套餐、公告、用户和多订阅后缀管理。
            </Paragraph>
          </div>
          <Button icon={<ReloadOutlined />} onClick={() => void loadStats()} loading={loading}>
            刷新统计
          </Button>
        </div>
      </Card>

      <Row gutter={[16, 16]}>
        <Col xs={24} md={12} xl={6}>
          <Card className="metric-card">
            <Statistic title="用户" value={stats?.users || 0} loading={loading} />
          </Card>
        </Col>
        <Col xs={24} md={12} xl={6}>
          <Card className="metric-card">
            <Statistic title="套餐" value={stats?.plans || 0} loading={loading} />
          </Card>
        </Col>
        <Col xs={24} md={12} xl={6}>
          <Card className="metric-card">
            <Statistic title="节点" value={stats?.servers || 0} loading={loading} />
          </Card>
        </Col>
        <Col xs={24} md={12} xl={6}>
          <Card className="metric-card">
            <Statistic title="公告" value={stats?.notices || 0} loading={loading} />
          </Card>
        </Col>
      </Row>

      <Row gutter={[16, 16]}>
        <Col xs={24} xl={12}>
          <Card className="panel-card" title="已启用能力">
            <List
              dataSource={[
                "管理员登录与安全路径",
                "用户 / 套餐 / 节点 / 公告管理",
                "一个用户多个订阅后缀",
                "默认 / Clash / V2 / Shadowrocket 订阅格式",
                "流量统计、到期时间和节点在线状态",
              ]}
              renderItem={(item) => <List.Item>{item}</List.Item>}
            />
          </Card>
        </Col>
        <Col xs={24} xl={12}>
          <Card className="panel-card" title="已移除模块">
            <Alert
              type="info"
              showIcon
              message="为了保持基础面板范围，以下复杂模块不再出现在 UI 中。"
              description="支付、订单、工单、优惠券、知识库、路由管理。"
            />
          </Card>
        </Col>
      </Row>
    </Space>
  );
}

function AdminUsersPage() {
  const { authData } = useSession();
  const [createForm] = Form.useForm<{ email?: string; password?: string; plan_id?: number }>();
  const [subscriptionForm] = Form.useForm<{ user_id?: number; name?: string; suffix?: string; is_primary?: boolean }>();
  const [users, setUsers] = useState<UserRow[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [activeUserId, setActiveUserId] = useState<number | null>(null);
  const [subscriptions, setSubscriptions] = useState<SubscriptionLink[]>([]);
  const [subscriptionLoading, setSubscriptionLoading] = useState(false);

  const loadUsers = useEffectEvent(async () => {
    setLoading(true);
    try {
      const payload = await requestJSON<{ data: UserRow[] }>(`${bootstrap.apiBase}/${bootstrap.adminPath}/user/fetch`, {
        authData,
      });
      setUsers(payload.data || []);
    } finally {
      setLoading(false);
    }
  });

  const loadSubscriptions = useEffectEvent(async (userID: number) => {
    setSubscriptionLoading(true);
    try {
      const payload = await requestJSON<ApiEnvelope<SubscriptionLink[]>>(
        `${bootstrap.apiBase}/${bootstrap.adminPath}/user/subscription/fetch?user_id=${userID}`,
        { authData },
      );
      setSubscriptions(unwrapData(payload));
    } finally {
      setSubscriptionLoading(false);
    }
  });

  useEffect(() => {
    void loadUsers();
  }, []);

  const createUser = async (values: { email?: string; password?: string; plan_id?: number }) => {
    setSaving(true);
    try {
      await requestJSON<ApiEnvelope<boolean>>(`${bootstrap.apiBase}/${bootstrap.adminPath}/user/generate`, {
        method: "POST",
        authData,
        body: values,
      });
      message.success("用户已创建");
      createForm.resetFields();
      await loadUsers();
    } finally {
      setSaving(false);
    }
  };

  const toggleBan = async (row: UserRow) => {
    await requestJSON<ApiEnvelope<boolean>>(`${bootstrap.apiBase}/${bootstrap.adminPath}/user/ban`, {
      method: "POST",
      authData,
      body: { id: row.id },
    });
    message.success("用户状态已更新");
    await loadUsers();
  };

  const resetSecret = async (row: UserRow) => {
    await requestJSON<ApiEnvelope<boolean>>(`${bootstrap.apiBase}/${bootstrap.adminPath}/user/resetSecret`, {
      method: "POST",
      authData,
      body: { id: row.id },
    });
    message.success("订阅信息已重置");
    await loadUsers();
    if (activeUserId === row.id) {
      await loadSubscriptions(row.id);
    }
  };

  const openSubscriptionDrawer = async (row: UserRow) => {
    setActiveUserId(row.id);
    subscriptionForm.setFieldsValue({ user_id: row.id, is_primary: false });
    setDrawerOpen(true);
    await loadSubscriptions(row.id);
  };

  const saveSubscription = async (values: {
    user_id?: number;
    name?: string;
    suffix?: string;
    is_primary?: boolean;
  }) => {
    if (!values.user_id) {
      return;
    }
    setSaving(true);
    try {
      await requestJSON<ApiEnvelope<boolean>>(
        `${bootstrap.apiBase}/${bootstrap.adminPath}/user/subscription/save`,
        {
          method: "POST",
          authData,
          body: {
            ...values,
            is_primary: Boolean(values.is_primary),
          },
        },
      );
      message.success("订阅后缀已保存");
      subscriptionForm.setFieldsValue({
        user_id: values.user_id,
        name: undefined,
        suffix: undefined,
        is_primary: false,
      });
      await loadSubscriptions(values.user_id);
      await loadUsers();
    } finally {
      setSaving(false);
    }
  };

  const resetSubscriptions = async () => {
    if (!activeUserId) {
      return;
    }
    await requestJSON<ApiEnvelope<{ count: number }>>(
      `${bootstrap.apiBase}/${bootstrap.adminPath}/user/subscription/reset`,
      {
        method: "POST",
        authData,
        body: { user_id: activeUserId },
      },
    );
    message.success("该用户全部订阅链接已重置");
    await loadSubscriptions(activeUserId);
    await loadUsers();
  };

  const dropSubscription = async (id: number) => {
    await requestJSON<ApiEnvelope<boolean>>(`${bootstrap.apiBase}/${bootstrap.adminPath}/user/subscription/drop`, {
      method: "POST",
      authData,
      body: { id },
    });
    message.success("订阅后缀已删除");
    if (activeUserId) {
      await loadSubscriptions(activeUserId);
    }
    await loadUsers();
  };

  return (
    <Space direction="vertical" size={18} style={{ width: "100%" }}>
      <Row gutter={[16, 16]}>
        <Col xs={24} xl={8}>
          <Card className="panel-card" title="生成用户">
            <Form layout="vertical" form={createForm} onFinish={createUser}>
              <Form.Item name="email" label="邮箱">
                <Input placeholder="new@example.com" />
              </Form.Item>
              <Form.Item name="password" label="密码">
                <Input placeholder="ChangeMe123!" />
              </Form.Item>
              <Form.Item name="plan_id" label="套餐 ID">
                <Input type="number" placeholder="1" />
              </Form.Item>
              <Button type="primary" htmlType="submit" loading={saving}>
                创建用户
              </Button>
            </Form>
          </Card>
        </Col>
        <Col xs={24} xl={16}>
          <Card className="panel-card" title="用户列表">
            <Table
              rowKey="id"
              loading={loading}
              dataSource={users}
              pagination={false}
              scroll={{ x: 1180 }}
              columns={[
                {
                  title: "用户",
                  dataIndex: "email",
                  render: (_, row: UserRow) => (
                    <Space direction="vertical" size={2}>
                      <Text strong>{row.email}</Text>
                      <Text type="secondary">{`ID ${row.id} · 创建于 ${formatDateTime(row.created_at)}`}</Text>
                    </Space>
                  ),
                },
                {
                  title: "套餐 / 状态",
                  dataIndex: "plan_name",
                  render: (_, row: UserRow) => (
                    <Space direction="vertical" size={4}>
                      <Text>{row.plan_name || "未分配套餐"}</Text>
                      <Space wrap>
                        <Badge status={row.banned ? "error" : "success"} text={row.banned ? "已禁用" : "正常"} />
                        {row.is_admin && <Tag color="gold">管理员</Tag>}
                      </Space>
                    </Space>
                  ),
                },
                {
                  title: "流量 / 到期",
                  dataIndex: "transfer_enable",
                  render: (_, row: UserRow) => (
                    <Space direction="vertical" size={2}>
                      <Text>{trafficSummary(row.u, row.d, row.transfer_enable)}</Text>
                      <Text type="secondary">{formatDateTime(row.expired_at)}</Text>
                    </Space>
                  ),
                },
                {
                  title: "主订阅",
                  dataIndex: "subscribe_suffix",
                  render: (_, row: UserRow) => (
                    <Space direction="vertical" size={2}>
                      <Text>{row.subscribe_suffix || "-"}</Text>
                      <Text type="secondary" ellipsis>
                        {row.subscribe_url || "暂无订阅地址"}
                      </Text>
                    </Space>
                  ),
                },
                {
                  title: "操作",
                  dataIndex: "actions",
                  fixed: "right",
                  render: (_, row: UserRow) => (
                    <Space wrap>
                      <Button size="small" onClick={() => void toggleBan(row)}>
                        {row.banned ? "解除禁用" : "禁用"}
                      </Button>
                      <Button size="small" onClick={() => void resetSecret(row)}>
                        重置订阅
                      </Button>
                      <Button size="small" type="primary" onClick={() => void openSubscriptionDrawer(row)}>
                        管理后缀
                      </Button>
                    </Space>
                  ),
                },
              ]}
            />
          </Card>
        </Col>
      </Row>

      <Drawer
        width={560}
        title={activeUserId ? `用户 ${activeUserId} 的订阅后缀` : "订阅后缀"}
        open={drawerOpen}
        onClose={() => setDrawerOpen(false)}
        extra={
          <Space>
            <Button onClick={() => void (activeUserId ? loadSubscriptions(activeUserId) : Promise.resolve())}>刷新</Button>
            <Button onClick={() => void resetSubscriptions()} disabled={!activeUserId}>
              重置全部订阅
            </Button>
          </Space>
        }
      >
        <Space direction="vertical" size={16} style={{ width: "100%" }}>
          <Form layout="vertical" form={subscriptionForm} onFinish={saveSubscription}>
            <Form.Item name="user_id" label="用户 ID" rules={[{ required: true }]}>
              <Input type="number" />
            </Form.Item>
            <Form.Item name="name" label="名称">
              <Input placeholder="Clash 主订阅" />
            </Form.Item>
            <Form.Item name="suffix" label="后缀">
              <Input placeholder="custom-sub-001" />
            </Form.Item>
            <Form.Item name="is_primary" label="设为主订阅" valuePropName="checked">
              <Switch />
            </Form.Item>
            <Button type="primary" htmlType="submit" loading={saving}>
              保存后缀
            </Button>
          </Form>
          <Divider />
          {subscriptionLoading ? (
            <div className="page-spin">
              <Spin />
            </div>
          ) : subscriptions.length === 0 ? (
            <Empty description="暂无订阅后缀" />
          ) : (
            <Space direction="vertical" size={12} style={{ width: "100%" }}>
              {subscriptions.map((item) => (
                <Card key={item.id} className="subscription-card">
                  <Space direction="vertical" size={10} style={{ width: "100%" }}>
                    <Space wrap>
                      <Text strong>{item.name}</Text>
                      <Tag>{item.suffix}</Tag>
                      {item.is_primary && <Tag color="blue">主订阅</Tag>}
                      <Tag>{item.enabled ? "启用" : "停用"}</Tag>
                    </Space>
                    <Paragraph copyable={{ text: item.urls.default }} className="subscription-code">
                      {item.urls.default}
                    </Paragraph>
                    <Space wrap>
                      <Button size="small" icon={<LinkOutlined />} href={item.urls.default} target="_blank">
                        默认
                      </Button>
                      <Button size="small" href={item.urls.v2} target="_blank">
                        V2
                      </Button>
                      <Button size="small" href={item.urls.clash} target="_blank">
                        Clash
                      </Button>
                      <Button size="small" href={item.urls.shadowrocket} target="_blank">
                        Shadowrocket
                      </Button>
                      <Popconfirm title="确认删除这个订阅后缀？" onConfirm={() => void dropSubscription(item.id)}>
                        <Button size="small" danger>
                          删除
                        </Button>
                      </Popconfirm>
                    </Space>
                    <Text type="secondary">{`最近使用 ${formatRelativeStamp(item.last_used_at)}`}</Text>
                  </Space>
                </Card>
              ))}
            </Space>
          )}
        </Space>
      </Drawer>
    </Space>
  );
}

function AdminPlansPage() {
  const { authData } = useSession();
  const [form] = Form.useForm<Plan>();
  const [plans, setPlans] = useState<Plan[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const loadPlans = useEffectEvent(async () => {
    setLoading(true);
    try {
      const payload = await requestJSON<ApiEnvelope<Plan[]>>(`${bootstrap.apiBase}/${bootstrap.adminPath}/plan/fetch`, {
        authData,
      });
      setPlans(unwrapData(payload));
    } finally {
      setLoading(false);
    }
  });

  useEffect(() => {
    void loadPlans();
  }, []);

  const savePlan = async (values: Partial<Plan>) => {
    setSaving(true);
    try {
      await requestJSON<ApiEnvelope<boolean>>(`${bootstrap.apiBase}/${bootstrap.adminPath}/plan/save`, {
        method: "POST",
        authData,
        body: values,
      });
      message.success("套餐已保存");
      form.resetFields();
      await loadPlans();
    } finally {
      setSaving(false);
    }
  };

  const dropPlan = async (id: number) => {
    await requestJSON<ApiEnvelope<boolean>>(`${bootstrap.apiBase}/${bootstrap.adminPath}/plan/drop`, {
      method: "POST",
      authData,
      body: { id },
    });
    message.success("套餐已删除");
    await loadPlans();
  };

  return (
    <Row gutter={[16, 16]}>
      <Col xs={24} xl={8}>
        <Card className="panel-card" title="新套餐">
          <Form layout="vertical" form={form} onFinish={savePlan} initialValues={{ price: 19.9, transfer_enable: 137438953472, speed_limit: 100 }}>
            <Form.Item name="name" label="名称" rules={[{ required: true }]}>
              <Input />
            </Form.Item>
            <Form.Item name="price" label="价格">
              <Input type="number" step="0.1" />
            </Form.Item>
            <Form.Item name="transfer_enable" label="流量字节">
              <Input type="number" />
            </Form.Item>
            <Form.Item name="speed_limit" label="速率限制">
              <Input type="number" />
            </Form.Item>
            <Form.Item name="content" label="简介">
              <Input.TextArea rows={5} />
            </Form.Item>
            <Button type="primary" htmlType="submit" loading={saving}>
              保存套餐
            </Button>
          </Form>
        </Card>
      </Col>
      <Col xs={24} xl={16}>
        <Card className="panel-card" title="套餐列表">
          <Table
            rowKey="id"
            loading={loading}
            dataSource={plans}
            pagination={false}
            scroll={{ x: 920 }}
            columns={[
              {
                title: "套餐",
                dataIndex: "name",
                render: (_, row: Plan) => (
                  <Space direction="vertical" size={2}>
                    <Text strong>{row.name}</Text>
                    <Text type="secondary">{`ID ${row.id}`}</Text>
                  </Space>
                ),
              },
              {
                title: "流量 / 速率",
                dataIndex: "transfer_enable",
                render: (_, row: Plan) => (
                  <Space direction="vertical" size={2}>
                    <Text>{formatBytes(row.transfer_enable)}</Text>
                    <Text type="secondary">{`${row.speed_limit} Mbps`}</Text>
                  </Space>
                ),
              },
              {
                title: "价格 / 用户",
                dataIndex: "price",
                render: (_, row: Plan) => (
                  <Space direction="vertical" size={2}>
                    <Text>{formatPrice(row.price)}</Text>
                    <Text type="secondary">{`${row.count || 0} 用户`}</Text>
                  </Space>
                ),
              },
              {
                title: "简介",
                dataIndex: "content",
                render: (value: string) => <Text type="secondary">{value || "暂无描述"}</Text>,
              },
              {
                title: "操作",
                dataIndex: "actions",
                render: (_, row: Plan) => (
                  <Popconfirm title="确认删除这个套餐？" onConfirm={() => void dropPlan(row.id)}>
                    <Button danger size="small">
                      删除
                    </Button>
                  </Popconfirm>
                ),
              },
            ]}
          />
        </Card>
      </Col>
    </Row>
  );
}

function AdminServersPage() {
  const { authData } = useSession();
  const [form] = Form.useForm<Partial<ServerNode> & { type?: string; tags?: string; plan_ids?: string }>();
  const [servers, setServers] = useState<ServerNode[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const loadServers = useEffectEvent(async () => {
    setLoading(true);
    try {
      const payload = await requestJSON<ApiEnvelope<ServerNode[]>>(
        `${bootstrap.apiBase}/${bootstrap.adminPath}/server/manage/getNodes`,
        { authData },
      );
      setServers(unwrapData(payload));
    } finally {
      setLoading(false);
    }
  });

  useEffect(() => {
    form.setFieldsValue({ type: "vmess", port: 443, network: "ws" });
    void loadServers();
  }, [form]);

  const saveServer = async (values: Partial<ServerNode> & { type?: string; tags?: string; plan_ids?: string }) => {
    if (!values.type) {
      return;
    }
    setSaving(true);
    try {
      const { type, ...rest } = values;
      await requestJSON<ApiEnvelope<boolean>>(`${bootstrap.apiBase}/${bootstrap.adminPath}/server/${type}/save`, {
        method: "POST",
        authData,
        body: rest,
      });
      message.success("节点已保存");
      form.resetFields();
      form.setFieldsValue({ type: "vmess", port: 443, network: "ws" });
      await loadServers();
    } finally {
      setSaving(false);
    }
  };

  const dropServer = async (row: ServerNode) => {
    await requestJSON<ApiEnvelope<boolean>>(
      `${bootstrap.apiBase}/${bootstrap.adminPath}/server/${row.type}/drop`,
      {
        method: "POST",
        authData,
        body: { id: row.id },
      },
    );
    message.success("节点已删除");
    await loadServers();
  };

  return (
    <Row gutter={[16, 16]}>
      <Col xs={24} xl={8}>
        <Card className="panel-card" title="新节点">
          <Form layout="vertical" form={form} onFinish={saveServer}>
            <Form.Item name="type" label="类型" rules={[{ required: true }]}>
              <Input placeholder="vmess / vless / trojan / shadowsocks / hysteria" />
            </Form.Item>
            <Form.Item name="name" label="名称" rules={[{ required: true }]}>
              <Input />
            </Form.Item>
            <Form.Item name="host" label="地址" rules={[{ required: true }]}>
              <Input />
            </Form.Item>
            <Form.Item name="port" label="端口">
              <Input type="number" />
            </Form.Item>
            <Form.Item name="network" label="网络">
              <Input />
            </Form.Item>
            <Form.Item name="tags" label="标签">
              <Input placeholder="Japan,Premium" />
            </Form.Item>
            <Form.Item name="plan_ids" label="套餐 ID">
              <Input placeholder="1,2" />
            </Form.Item>
            <Button type="primary" htmlType="submit" loading={saving}>
              保存节点
            </Button>
          </Form>
        </Card>
      </Col>
      <Col xs={24} xl={16}>
        <Card className="panel-card" title="节点列表">
          <Table
            rowKey="id"
            loading={loading}
            dataSource={servers}
            pagination={false}
            scroll={{ x: 980 }}
            columns={[
              {
                title: "节点",
                dataIndex: "name",
                render: (_, row: ServerNode) => (
                  <Space direction="vertical" size={2}>
                    <Text strong>{row.name}</Text>
                    <Text type="secondary">{`${row.type} · 倍率 ${row.rate}`}</Text>
                  </Space>
                ),
              },
              {
                title: "地址",
                dataIndex: "host",
                render: (_, row: ServerNode) => (
                  <Space direction="vertical" size={2}>
                    <Text>{`${row.host || "-"}:${row.port || 0}`}</Text>
                    <Text type="secondary">{row.network || "-"}</Text>
                  </Space>
                ),
              },
              {
                title: "标签 / 套餐",
                dataIndex: "tags",
                render: (_, row: ServerNode) => (
                  <Space direction="vertical" size={6}>
                    <Space wrap>{(row.tags || []).length ? row.tags?.map((item) => <Tag key={item}>{item}</Tag>) : <Text type="secondary">无标签</Text>}</Space>
                    <Text type="secondary">{`套餐 ${(row.plan_ids || []).join(", ") || "全部"}`}</Text>
                  </Space>
                ),
              },
              {
                title: "状态",
                dataIndex: "is_online",
                render: (_, row: ServerNode) => (
                  <Space direction="vertical" size={2}>
                    <Badge status={row.is_online ? "success" : "error"} text={row.is_online ? "在线" : "离线"} />
                    <Text type="secondary">{row.show ? "前台展示" : "前台隐藏"}</Text>
                  </Space>
                ),
              },
              {
                title: "操作",
                dataIndex: "actions",
                render: (_, row: ServerNode) => (
                  <Popconfirm title="确认删除这个节点？" onConfirm={() => void dropServer(row)}>
                    <Button danger size="small">
                      删除
                    </Button>
                  </Popconfirm>
                ),
              },
            ]}
          />
        </Card>
      </Col>
    </Row>
  );
}

function AdminNoticesPage() {
  const { authData } = useSession();
  const [form] = Form.useForm<{ title?: string; content?: string }>();
  const [notices, setNotices] = useState<Notice[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const loadNotices = useEffectEvent(async () => {
    setLoading(true);
    try {
      const payload = await requestJSON<{ data: Notice[] }>(`${bootstrap.apiBase}/${bootstrap.adminPath}/notice/fetch`, {
        authData,
      });
      setNotices(payload.data || []);
    } finally {
      setLoading(false);
    }
  });

  useEffect(() => {
    void loadNotices();
  }, []);

  const saveNotice = async (values: { title?: string; content?: string }) => {
    setSaving(true);
    try {
      await requestJSON<ApiEnvelope<boolean>>(`${bootstrap.apiBase}/${bootstrap.adminPath}/notice/save`, {
        method: "POST",
        authData,
        body: values,
      });
      message.success("公告已发布");
      form.resetFields();
      await loadNotices();
    } finally {
      setSaving(false);
    }
  };

  const dropNotice = async (id: number) => {
    await requestJSON<ApiEnvelope<boolean>>(`${bootstrap.apiBase}/${bootstrap.adminPath}/notice/drop`, {
      method: "POST",
      authData,
      body: { id },
    });
    message.success("公告已删除");
    await loadNotices();
  };

  return (
    <Row gutter={[16, 16]}>
      <Col xs={24} xl={8}>
        <Card className="panel-card" title="新公告">
          <Form layout="vertical" form={form} onFinish={saveNotice}>
            <Form.Item name="title" label="标题" rules={[{ required: true }]}>
              <Input />
            </Form.Item>
            <Form.Item name="content" label="内容">
              <Input.TextArea rows={6} />
            </Form.Item>
            <Button type="primary" htmlType="submit" loading={saving}>
              发布公告
            </Button>
          </Form>
        </Card>
      </Col>
      <Col xs={24} xl={16}>
        <Card className="panel-card" title="公告列表">
          {loading ? (
            <div className="page-spin">
              <Spin />
            </div>
          ) : notices.length === 0 ? (
            <Empty description="暂无公告" />
          ) : (
            <Space direction="vertical" size={12} style={{ width: "100%" }}>
              {notices.map((notice) => (
                <Card key={notice.id} className="notice-card">
                  <div className="card-row-between">
                    <div>
                      <Title level={5}>{notice.title}</Title>
                      <Text type="secondary">{formatDateTime(notice.created_at)}</Text>
                    </div>
                    <Popconfirm title="确认删除这条公告？" onConfirm={() => void dropNotice(notice.id)}>
                      <Button danger size="small">
                        删除
                      </Button>
                    </Popconfirm>
                  </div>
                  <Paragraph>{notice.content || "暂无内容"}</Paragraph>
                </Card>
              ))}
            </Space>
          )}
        </Card>
      </Col>
    </Row>
  );
}

function AdminSettingsPage() {
  const { authData } = useSession();
  const [form] = Form.useForm<SettingsRecord>();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);

  const loadSettings = useEffectEvent(async () => {
    setLoading(true);
    try {
      const payload = await requestJSON<ApiEnvelope<SettingsRecord>>(
        `${bootstrap.apiBase}/${bootstrap.adminPath}/config/fetch`,
        { authData },
      );
      form.setFieldsValue(unwrapData(payload));
    } finally {
      setLoading(false);
    }
  });

  useEffect(() => {
    void loadSettings();
  }, []);

  const saveSettings = async (values: SettingsRecord) => {
    setSaving(true);
    try {
      await requestJSON<ApiEnvelope<boolean>>(`${bootstrap.apiBase}/${bootstrap.adminPath}/config/save`, {
        method: "POST",
        authData,
        body: values,
      });
      message.success("配置已保存");
      await loadSettings();
    } finally {
      setSaving(false);
    }
  };

  return (
    <Card className="panel-card" title="站点设置">
      <Form layout="vertical" form={form} onFinish={saveSettings}>
        <Row gutter={[16, 0]}>
          <Col xs={24} xl={12}>
            <Form.Item name="app_name" label="站点名称">
              <Input />
            </Form.Item>
          </Col>
          <Col xs={24} xl={12}>
            <Form.Item name="app_description" label="描述">
              <Input />
            </Form.Item>
          </Col>
          <Col xs={24} xl={12}>
            <Form.Item name="app_url" label="公共地址">
              <Input />
            </Form.Item>
          </Col>
          <Col xs={24} xl={12}>
            <Form.Item name="subscribe_url" label="订阅地址基准">
              <Input />
            </Form.Item>
          </Col>
          <Col xs={24} xl={12}>
            <Form.Item name="secure_path" label="安全路径">
              <Input />
            </Form.Item>
          </Col>
          <Col xs={24} xl={12}>
            <Form.Item name="server_token" label="节点对接 Token">
              <Input />
            </Form.Item>
          </Col>
          <Col xs={24} xl={12}>
            <Form.Item name="logo" label="Logo 文本">
              <Input />
            </Form.Item>
          </Col>
        </Row>
        <Space>
          <Button type="primary" htmlType="submit" loading={saving}>
            保存配置
          </Button>
          {loading && <Spin size="small" />}
        </Space>
      </Form>
    </Card>
  );
}

function SubscriptionCard({ item }: { item: SubscriptionLink }) {
  return (
    <Card className="subscription-card">
      <Space direction="vertical" size={10} style={{ width: "100%" }}>
        <Space wrap>
          <Text strong>{item.name}</Text>
          <Tag>{item.suffix}</Tag>
          {item.is_primary && <Tag color="blue">主订阅</Tag>}
          <Tag>{item.enabled ? "启用" : "停用"}</Tag>
        </Space>
        <Paragraph copyable={{ text: item.urls.default }} className="subscription-code">
          {item.urls.default}
        </Paragraph>
        <Space wrap>
          <Button size="small" icon={<LinkOutlined />} href={item.urls.default} target="_blank">
            默认
          </Button>
          <Button size="small" href={item.urls.v2} target="_blank">
            V2
          </Button>
          <Button size="small" href={item.urls.clash} target="_blank">
            Clash
          </Button>
          <Button size="small" href={item.urls.shadowrocket} target="_blank">
            Shadowrocket
          </Button>
        </Space>
        <Text type="secondary">{`最近使用 ${formatRelativeStamp(item.last_used_at)}`}</Text>
      </Space>
    </Card>
  );
}

function FullscreenLoading({ title }: { title: string }) {
  return (
    <div className="fullscreen-loading">
      <Spin size="large" />
      <Text>{title}</Text>
    </div>
  );
}
